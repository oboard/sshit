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
	Type     string          `json:"type"`
	ID       int             `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	X        int             `json:"x,omitempty"`
	Y        int             `json:"y,omitempty"`
	Width    int             `json:"width,omitempty"`
	Height   int             `json:"height,omitempty"`
	Cols     uint16          `json:"cols,omitempty"`
	Rows     uint16          `json:"rows,omitempty"`
	Data     string          `json:"data,omitempty"`
	Password string          `json:"password,omitempty"`
	Update   string          `json:"update,omitempty"`
	Users    []webUser       `json:"users"`
	Shells   []webShellState `json:"shells"`
	User     *webUser        `json:"user,omitempty"`
	Shell    *webShellState  `json:"shell,omitempty"`
}

type webUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Cursor bool   `json:"cursor"`
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

type collabClient struct {
	conn          *websocket.Conn
	send          chan []byte
	authenticated bool
	closed        bool
}

type webHub struct {
	mu            sync.Mutex
	nextID        int
	clients       map[int]*webClient
	shells        map[int]*webShell
	shellSeq      int
	password      string
	collabClients map[*collabClient]bool
	collabUpdates [][]byte
}

func newWebHub(password string) *webHub {
	return &webHub{
		clients:       make(map[int]*webClient),
		shells:        make(map[int]*webShell),
		password:      password,
		collabClients: make(map[*collabClient]bool),
	}
}

func (h *webHub) snapshotLocked() (users []webUser, shells []webShellState) {
	users = make([]webUser, 0, len(h.clients))
	shells = make([]webShellState, 0, len(h.shells))
	for _, c := range h.clients {
		users = append(users, webUser{ID: c.id, Name: c.name, X: c.x, Y: c.y, Cursor: true})
	}
	for _, s := range h.shells {
		state := s.webShellState
		state.Buffer = string(s.buffer)
		shells = append(shells, state)
	}
	return users, shells
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
	users, shells := h.snapshotLocked()
	h.mu.Unlock()
	h.broadcast(wsEnvelope{Type: "state", Users: users, Shells: shells})
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
	users, shells := h.snapshotLocked()
	h.mu.Unlock()
	if client.authenticated {
		client.send <- wsEnvelope{Type: "hello", ID: client.id, Users: users, Shells: shells}
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

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *webHub) addCollabClient(client *collabClient) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.collabClients[client] = true
	updates := make([][]byte, len(h.collabUpdates))
	for i, update := range h.collabUpdates {
		updates[i] = append([]byte(nil), update...)
	}
	return updates
}

func (h *webHub) removeCollabClient(client *collabClient) {
	h.mu.Lock()
	if _, ok := h.collabClients[client]; ok {
		delete(h.collabClients, client)
	}
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
		case client.send <- stored:
		default:
			go h.removeCollabClient(client)
		}
	}
	h.mu.Unlock()
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

		client := &collabClient{conn: conn, send: make(chan []byte, 256)}
		defer hub.removeCollabClient(client)

		go func() {
			for update := range client.send {
				if err := conn.WriteMessage(websocket.BinaryMessage, update); err != nil {
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
					_ = conn.WriteJSON(wsEnvelope{Type: "authFailed"})
					return
				}
				client.authenticated = true
				updates := hub.addCollabClient(client)
				_ = conn.WriteJSON(wsEnvelope{Type: "ready", ID: len(updates)})
				for _, update := range updates {
					client.send <- update
				}
				continue
			}

			if messageType == websocket.BinaryMessage && len(payload) > 0 {
				hub.broadcastCollabUpdate(client, payload)
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
				users, shells := hub.snapshotLocked()
				hub.mu.Unlock()
				client.send <- wsEnvelope{Type: "hello", ID: client.id, Users: users, Shells: shells}
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
				user := webUser{ID: client.id, Name: client.name, X: client.x, Y: client.y, Cursor: true}
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
