package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sshit/internal/web"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

func defaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell
}

func terminalEnv(term string) []string {
	if term == "" || term == "dumb" {
		term = "xterm-256color"
	}
	env := os.Environ()
	env = appendEnv(env, "TERM", term)
	env = appendEnv(env, "COLORTERM", "truecolor")
	env = appendEnv(env, "TERM_PROGRAM", "sshit")
	env = removeEnv(env, "TERM_PROGRAM_VERSION")
	return env
}

func appendEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func removeEnv(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

func hostKeyPath() (string, error) {
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if home, err := os.UserHomeDir(); err == nil {
		sshDir = filepath.Join(home, ".ssh")
	}
	return filepath.Join(sshDir, "sshit_host_ed25519_key"), os.MkdirAll(sshDir, 0700)
}

func loadOrCreateHostKey() (ssh.Signer, string, error) {
	path, err := hostKeyPath()
	if err != nil {
		return nil, "", err
	}

	if key, err := os.ReadFile(path); err == nil {
		signer, err := gossh.ParsePrivateKey(key)
		if err != nil {
			return nil, path, fmt.Errorf("parse host key: %w", err)
		}
		return signer, path, nil
	} else if !os.IsNotExist(err) {
		return nil, path, err
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, path, err
	}

	block, err := gossh.MarshalPrivateKey(privateKey, "sshit host key")
	if err != nil {
		return nil, path, err
	}

	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, path, err
	}

	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, path, err
	}
	return signer, path, nil
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

type singleConnListener struct {
	conn chan net.Conn
}

func newSingleConnListener(c net.Conn) *singleConnListener {
	l := &singleConnListener{conn: make(chan net.Conn, 1)}
	l.conn <- c
	close(l.conn)
	return l
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	conn, ok := <-l.conn
	if !ok {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (l *singleConnListener) Close() error { return nil }

func (l *singleConnListener) Addr() net.Addr { return dummyAddr("single-connection") }

type dummyAddr string

func (a dummyAddr) Network() string { return string(a) }

func (a dummyAddr) String() string { return string(a) }

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (l tcpKeepAliveListener) Accept() (net.Conn, error) {
	conn, err := l.AcceptTCP()
	if err != nil {
		return nil, err
	}
	_ = conn.SetKeepAlive(true)
	_ = conn.SetKeepAlivePeriod(3 * time.Minute)
	return conn, nil
}

func isSSH(r *bufio.Reader) bool {
	prefix, err := r.Peek(4)
	return err == nil && string(prefix) == "SSH-"
}

func serveMux(listener net.Listener, sshServer *ssh.Server, httpServer *http.Server) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		go func(conn net.Conn) {
			reader := bufio.NewReader(conn)
			buffered := &bufferedConn{Conn: conn, r: reader}
			if isSSH(reader) {
				sshServer.HandleConn(buffered)
				return
			}

			if err := httpServer.Serve(newSingleConnListener(buffered)); err != nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("http connection error: %v", err)
			}
		}(conn)
	}
}

func shellHandler(s ssh.Session) {
	cmd := exec.Command(defaultShell())
	ptyReq, winCh, isPty := s.Pty()
	cmd.Env = terminalEnv(ptyReq.Term)

	if !isPty {
		io.WriteString(s, "PTY required\n")
		return
	}

	p, err := pty.StartWithAttrs(
		cmd,
		&pty.Winsize{
			Cols: uint16(ptyReq.Window.Width),
			Rows: uint16(ptyReq.Window.Height),
		},
		nil,
	)
	if err != nil {
		log.Printf("failed to start pty: %v", err)
		return
	}
	defer p.Close()

	go func() {
		for win := range winCh {
			_ = pty.Setsize(p, &pty.Winsize{
				Cols: uint16(win.Width),
				Rows: uint16(win.Height),
			})
		}
	}()

	go io.Copy(p, s)
	io.Copy(s, p)
}

type wsEnvelope struct {
	Type        string          `json:"type"`
	ID          int             `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Color       string          `json:"color,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	UserID      int             `json:"userId,omitempty"`
	Awareness   json.RawMessage `json:"awareness,omitempty"`
	X           int             `json:"x,omitempty"`
	Y           int             `json:"y,omitempty"`
	CursorStyle string          `json:"cursorStyle,omitempty"`
	Width       int             `json:"width,omitempty"`
	Height      int             `json:"height,omitempty"`
	Cols        uint16          `json:"cols,omitempty"`
	Rows        uint16          `json:"rows,omitempty"`
	Data        string          `json:"data,omitempty"`
	Password    string          `json:"password,omitempty"`
	Update      string          `json:"update,omitempty"`
	Users       []webUser       `json:"users"`
	Shells      []webShellState `json:"shells"`
	User        *webUser        `json:"user,omitempty"`
	Shell       *webShellState  `json:"shell,omitempty"`

	EditorWindows []editorWindowState `json:"editorWindows"`
	EditorWindow  *editorWindowState  `json:"editorWindow,omitempty"`
	Patch         *editorWindowPatch  `json:"patch,omitempty"`
	WindowID      int64               `json:"windowId,omitempty"`
}

type editorWindowState struct {
	ID     int64  `json:"id"`
	DocID  string `json:"docId"`
	Kind   string `json:"kind"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	ZIndex int    `json:"zIndex"`
}

type editorWindowPatch struct {
	X      *int `json:"x,omitempty"`
	Y      *int `json:"y,omitempty"`
	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`
	ZIndex *int `json:"zIndex,omitempty"`
}

type webUser struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Cursor      bool   `json:"cursor"`
	CursorStyle string `json:"cursorStyle,omitempty"`
}

type webShellState struct {
	ID     int    `json:"id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Cols   uint16 `json:"cols"`
	Rows   uint16 `json:"rows"`
	Buffer string `json:"buffer,omitempty"`
}

type webShell struct {
	webShellState
	pty    *os.File
	buffer []byte
}

type webClient struct {
	id            int
	name          string
	x             int
	y             int
	authenticated bool
	conn          *websocket.Conn
	send          chan wsEnvelope
	hub           *webHub
}

type collabMessage struct {
	messageType int
	payload     []byte
}

type collabClient struct {
	conn          *websocket.Conn
	send          chan collabMessage
	clientID      string
	userID        int
	name          string
	color         string
	awareness     json.RawMessage
	authenticated bool
	closed        bool
}

var collabColors = []string{"#f472b6", "#60a5fa", "#34d399", "#fbbf24", "#a78bfa", "#fb7185", "#22d3ee", "#fb923c"}

type webHub struct {
	mu            sync.Mutex
	nextID        int
	collabSeq     int
	clients       map[int]*webClient
	shells        map[int]*webShell
	shellSeq      int
	password      string
	collabClients map[*collabClient]bool
	collabUpdates [][]byte
	editorWindows map[int64]*editorWindowState
}

func newWebHub(password string) *webHub {
	return &webHub{
		clients:       make(map[int]*webClient),
		shells:        make(map[int]*webShell),
		password:      password,
		collabClients: make(map[*collabClient]bool),
		editorWindows: make(map[int64]*editorWindowState),
	}
}

func (h *webHub) snapshotLocked() (users []webUser, shells []webShellState, editorWindows []editorWindowState) {
	users = make([]webUser, 0, len(h.clients))
	shells = make([]webShellState, 0, len(h.shells))
	editorWindows = make([]editorWindowState, 0, len(h.editorWindows))
	for _, c := range h.clients {
		users = append(users, webUser{ID: c.id, Name: c.name, X: c.x, Y: c.y, Cursor: true})
	}
	for _, s := range h.shells {
		state := s.webShellState
		state.Buffer = string(s.buffer)
		shells = append(shells, state)
	}
	for _, w := range h.editorWindows {
		editorWindows = append(editorWindows, *w)
	}
	return users, shells, editorWindows
}

func (h *webHub) broadcast(msg wsEnvelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (h *webHub) broadcastState() {
	h.mu.Lock()
	users, shells, editorWindows := h.snapshotLocked()
	h.mu.Unlock()
	h.broadcast(wsEnvelope{Type: "state", Users: users, Shells: shells, EditorWindows: editorWindows})
}

func (h *webHub) addClient(conn *websocket.Conn) *webClient {
	h.mu.Lock()
	h.nextID++
	client := &webClient{
		id:            h.nextID,
		name:          fmt.Sprintf("user-%d", h.nextID),
		authenticated: h.password == "",
		conn:          conn,
		send:          make(chan wsEnvelope, 64),
		hub:           h,
	}
	if client.authenticated {
		h.clients[client.id] = client
	}
	users, shells, editorWindows := h.snapshotLocked()
	h.mu.Unlock()
	if client.authenticated {
		client.send <- wsEnvelope{Type: "hello", ID: client.id, Users: users, Shells: shells, EditorWindows: editorWindows}
		h.broadcastState()
	} else {
		client.send <- wsEnvelope{Type: "authRequired"}
	}
	return client
}

func (h *webHub) removeClient(id int) {
	h.mu.Lock()
	removed := false
	if c, ok := h.clients[id]; ok {
		delete(h.clients, id)
		close(c.send)
		removed = true
	}
	h.mu.Unlock()
	if removed {
		h.broadcastState()
	}
}

func (h *webHub) createShell(x, y int, cols, rows uint16) (*webShell, error) {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	cmd := exec.Command(defaultShell())
	cmd.Env = terminalEnv("xterm-256color")
	p, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	h.shellSeq++
	shell := &webShell{
		webShellState: webShellState{ID: h.shellSeq, X: x, Y: y, Width: 760, Height: 420, Cols: cols, Rows: rows},
		pty:           p,
	}
	h.shells[shell.ID] = shell
	h.mu.Unlock()

	go h.readShell(shell)
	h.broadcastState()
	return shell, nil
}

func (h *webHub) readShell(shell *webShell) {
	buf := make([]byte, 32*1024)
	for {
		n, err := shell.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			h.mu.Lock()
			shell.buffer = append(shell.buffer, chunk...)
			if len(shell.buffer) > 1<<20 {
				shell.buffer = shell.buffer[len(shell.buffer)-(1<<20):]
			}
			h.mu.Unlock()
			h.broadcast(wsEnvelope{Type: "output", ID: shell.ID, Data: string(chunk)})
		}
		if err != nil {
			h.closeShell(shell.ID)
			return
		}
	}
}

func (h *webHub) closeShell(id int) {
	h.mu.Lock()
	shell, ok := h.shells[id]
	if ok {
		delete(h.shells, id)
	}
	h.mu.Unlock()
	if ok {
		_ = shell.pty.Close()
		h.broadcastState()
	}
}

func (h *webHub) shell(id int) *webShell {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.shells[id]
}

func (h *webHub) createEditorWindow(state editorWindowState) {
	h.mu.Lock()
	stored := state
	h.editorWindows[state.ID] = &stored
	h.mu.Unlock()
	h.broadcast(wsEnvelope{Type: "editorWindowCreated", EditorWindow: &stored})
}

func (h *webHub) patchEditorWindow(id int64, patch editorWindowPatch) {
	h.mu.Lock()
	window, ok := h.editorWindows[id]
	if ok {
		if patch.X != nil {
			window.X = *patch.X
		}
		if patch.Y != nil {
			window.Y = *patch.Y
		}
		if patch.Width != nil {
			window.Width = *patch.Width
		}
		if patch.Height != nil {
			window.Height = *patch.Height
		}
		if patch.ZIndex != nil {
			window.ZIndex = *patch.ZIndex
		}
	}
	h.mu.Unlock()
	if ok {
		h.broadcast(wsEnvelope{Type: "editorWindowPatched", WindowID: id, Patch: &patch})
	}
}

func (h *webHub) closeEditorWindow(id int64) {
	h.mu.Lock()
	_, ok := h.editorWindows[id]
	if ok {
		delete(h.editorWindows, id)
	}
	h.mu.Unlock()
	if ok {
		h.broadcast(wsEnvelope{Type: "editorWindowClosed", WindowID: id})
	}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *webHub) sendCollabJSON(client *collabClient, message wsEnvelope) bool {
	payload, err := json.Marshal(message)
	if err != nil {
		return false
	}
	select {
	case client.send <- collabMessage{messageType: websocket.TextMessage, payload: payload}:
		return true
	default:
		return false
	}
}

func (h *webHub) addCollabClient(client *collabClient) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.collabSeq++
	client.clientID = fmt.Sprintf("collab-%d", h.collabSeq)
	colorIndex := h.collabSeq - 1
	if client.userID > 0 {
		colorIndex = client.userID - 1
	}
	client.color = collabColors[colorIndex%len(collabColors)]

	// Queue the complete history and ready marker before making this client
	// visible to live broadcasts. The single writer goroutine preserves this
	// exact order on the WebSocket.
	for _, update := range h.collabUpdates {
		copied := append([]byte(nil), update...)
		select {
		case client.send <- collabMessage{messageType: websocket.BinaryMessage, payload: copied}:
		default:
			return false
		}
	}
	ready, err := json.Marshal(wsEnvelope{Type: "ready", ID: len(h.collabUpdates), ClientID: client.clientID, Name: client.name, Color: client.color})
	if err != nil {
		return false
	}
	select {
	case client.send <- collabMessage{messageType: websocket.TextMessage, payload: ready}:
	default:
		return false
	}
	for other := range h.collabClients {
		if len(other.awareness) == 0 || string(other.awareness) == "null" {
			continue
		}
		payload, err := json.Marshal(wsEnvelope{
			Type: "awareness", ClientID: other.clientID, Name: other.name, Color: other.color, Awareness: other.awareness,
		})
		if err != nil {
			continue
		}
		select {
		case client.send <- collabMessage{messageType: websocket.TextMessage, payload: payload}:
		default:
			return false
		}
	}
	h.collabClients[client] = true
	return true
}

func (h *webHub) removeCollabClient(client *collabClient) {
	h.mu.Lock()
	delete(h.collabClients, client)
	if !client.closed {
		client.closed = true
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *webHub) broadcastCollabUpdate(sender *collabClient, update []byte) {
	h.mu.Lock()
	stored := append([]byte(nil), update...)
	h.collabUpdates = append(h.collabUpdates, stored)
	for client := range h.collabClients {
		if client == sender {
			continue
		}
		select {
		case client.send <- collabMessage{messageType: websocket.BinaryMessage, payload: stored}:
		default:
			go h.removeCollabClient(client)
		}
	}
	h.mu.Unlock()
}

func (h *webHub) broadcastCollabAwareness(sender *collabClient, awareness json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sender.awareness = append(sender.awareness[:0], awareness...)
	payload, err := json.Marshal(wsEnvelope{
		Type: "awareness", ClientID: sender.clientID, Name: sender.name, Color: sender.color, Awareness: awareness,
	})
	if err != nil {
		return
	}
	for client := range h.collabClients {
		if client == sender {
			continue
		}
		select {
		case client.send <- collabMessage{messageType: websocket.TextMessage, payload: payload}:
		default:
			go h.removeCollabClient(client)
		}
	}
}

func (h *webHub) clearCollabAwareness(sender *collabClient) {
	if sender.clientID == "" {
		return
	}
	h.broadcastCollabAwareness(sender, json.RawMessage("null"))
}

func webSocketCollab(hub *webHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("collab websocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		conn.SetReadLimit(4 << 20)

		client := &collabClient{conn: conn, send: make(chan collabMessage, 256)}
		defer hub.removeCollabClient(client)
		defer hub.clearCollabAwareness(client)

		go func() {
			for message := range client.send {
				if err := conn.WriteMessage(message.messageType, message.payload); err != nil {
					return
				}
			}
		}()

		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if !client.authenticated {
				if messageType != websocket.TextMessage {
					continue
				}
				var msg wsEnvelope
				if err := json.Unmarshal(payload, &msg); err != nil || msg.Type != "auth" {
					continue
				}
				if hub.password != "" && msg.Password != hub.password {
					_ = hub.sendCollabJSON(client, wsEnvelope{Type: "authFailed"})
					return
				}
				client.name = msg.Name
				client.userID = msg.UserID
				if !hub.addCollabClient(client) {
					return
				}
				client.authenticated = true
				continue
			}

			if messageType == websocket.BinaryMessage && len(payload) > 0 {
				hub.broadcastCollabUpdate(client, payload)
				continue
			}
			if messageType == websocket.TextMessage {
				var msg wsEnvelope
				if err := json.Unmarshal(payload, &msg); err != nil {
					continue
				}
				if msg.Type == "awareness" {
					hub.broadcastCollabAwareness(client, msg.Awareness)
				}
			}
		}
	}
}

func webSocketShell(hub *webHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}
		client := hub.addClient(conn)
		defer conn.Close()
		defer hub.removeClient(client.id)

		go func() {
			for msg := range client.send {
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
			}
		}()

		for {
			var msg wsEnvelope
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}

			switch msg.Type {
			case "auth":
				if hub.password != "" && msg.Password != hub.password {
					client.send <- wsEnvelope{Type: "authFailed"}
					continue
				}
				hub.mu.Lock()
				if !client.authenticated {
					client.authenticated = true
					hub.clients[client.id] = client
				}
				users, shells, editorWindows := hub.snapshotLocked()
				hub.mu.Unlock()
				client.send <- wsEnvelope{Type: "hello", ID: client.id, Users: users, Shells: shells, EditorWindows: editorWindows}
				hub.broadcastState()
			case "setName":
				if !client.authenticated {
					continue
				}
				hub.mu.Lock()
				client.name = msg.Name
				hub.mu.Unlock()
				hub.broadcastState()
			case "cursor":
				if !client.authenticated {
					continue
				}
				hub.mu.Lock()
				client.x, client.y = msg.X, msg.Y
				user := webUser{ID: client.id, Name: client.name, X: client.x, Y: client.y, Cursor: true, CursorStyle: msg.CursorStyle}
				hub.mu.Unlock()
				hub.broadcast(wsEnvelope{Type: "cursor", User: &user})
			case "create":
				if !client.authenticated {
					continue
				}
				if _, err := hub.createShell(msg.X, msg.Y, msg.Cols, msg.Rows); err != nil {
					log.Printf("failed to create web shell: %v", err)
				}
			case "input":
				if !client.authenticated {
					continue
				}
				if shell := hub.shell(msg.ID); shell != nil {
					_, _ = shell.pty.Write([]byte(msg.Data))
				}
			case "resize":
				if !client.authenticated {
					continue
				}
				if shell := hub.shell(msg.ID); shell != nil {
					if msg.Width > 0 {
						shell.Width = msg.Width
					}
					if msg.Height > 0 {
						shell.Height = msg.Height
					}
					shell.Cols, shell.Rows = msg.Cols, msg.Rows
					_ = pty.Setsize(shell.pty, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
					hub.broadcastState()
				}
			case "move":
				if !client.authenticated {
					continue
				}
				hub.mu.Lock()
				if shell := hub.shells[msg.ID]; shell != nil {
					shell.X, shell.Y = msg.X, msg.Y
				}
				hub.mu.Unlock()
				hub.broadcastState()
			case "close":
				if !client.authenticated {
					continue
				}
				hub.closeShell(msg.ID)
			case "editorWindowCreate":
				if !client.authenticated {
					continue
				}
				if msg.EditorWindow != nil {
					hub.createEditorWindow(*msg.EditorWindow)
				}
			case "editorWindowPatch":
				if !client.authenticated {
					continue
				}
				if msg.Patch != nil {
					hub.patchEditorWindow(msg.WindowID, *msg.Patch)
				}
			case "editorWindowClose":
				if !client.authenticated {
					continue
				}
				hub.closeEditorWindow(msg.WindowID)
			}
		}
	}
}

func newHTTPHandler(hub *webHub) (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}

	files := http.FileServer(http.FS(dist))
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", webSocketShell(hub))
	mux.HandleFunc("/collab", webSocketCollab(hub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/collab") {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	return mux, nil
}

func main() {
	port := flag.Int("port", 2222, "port to listen on")
	flag.IntVar(port, "p", 2222, "port to listen on")
	password := flag.String("password", "", "password required for SSH and Web UI access")
	flag.Parse()

	hostKey, hostKeyPath, err := loadOrCreateHostKey()
	if err != nil {
		log.Fatalf("failed to load or create host key: %v", err)
	}

	sshServer := &ssh.Server{
		Handler:     shellHandler,
		HostSigners: []ssh.Signer{hostKey},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session": ssh.DefaultSessionHandler,
		},
	}
	if *password != "" {
		sshServer.PasswordHandler = func(ctx ssh.Context, candidate string) bool {
			return candidate == *password
		}
	}

	hub := newWebHub(*password)
	if _, err := hub.createShell(0, 0, 80, 24); err != nil {
		log.Printf("failed to create initial web shell: %v", err)
	}

	httpHandler, err := newHTTPHandler(hub)
	if err != nil {
		log.Fatalf("failed to load web UI: %v", err)
	}
	httpServer := &http.Server{Handler: httpHandler}

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		listener = tcpKeepAliveListener{tcpListener}
	}

	log.Printf("using host key %s", hostKeyPath)
	log.Printf("listening on %s for SSH and HTTP", addr)
	log.Fatal(serveMux(listener, sshServer, httpServer))
}
