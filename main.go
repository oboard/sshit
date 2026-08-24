package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sshit/internal/persist"
	"sshit/internal/update"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

// version is injected at build time via -ldflags "-X main.version=..." so that
// every release binary reports the tag it was built from (see scripts/release).
var version = "0.0.0-dev"

const (
	repoOwner = "oboard"
	repoName  = "sshit"
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
	ID          int64           `json:"id,omitempty"`
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
	Kind        string          `json:"kind,omitempty"`
	DocID       string          `json:"docId,omitempty"`
	Users       []webUser       `json:"users"`
	Windows     []windowState   `json:"windows"`
	User        *webUser        `json:"user,omitempty"`

	Patch    *windowPatch `json:"patch,omitempty"`
	WindowID int64        `json:"windowId,omitempty"`
}

// windowPatch carries optional updates to a window's geometry, z-order and
// (for shells) terminal size. Nil fields are left unchanged.
type windowPatch struct {
	X      *int    `json:"x,omitempty"`
	Y      *int    `json:"y,omitempty"`
	Width  *int    `json:"width,omitempty"`
	Height *int    `json:"height,omitempty"`
	ZIndex *int    `json:"zIndex,omitempty"`
	Cols   *uint16 `json:"cols,omitempty"`
	Rows   *uint16 `json:"rows,omitempty"`
}

type webUser struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Cursor      bool   `json:"cursor"`
	CursorStyle string `json:"cursorStyle,omitempty"`
}

// windowState is the serializable state of one workspace window. Shell and
// editor windows share this shape and are told apart by Kind.
type windowState struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	ZIndex int    `json:"zIndex"`

	// Shell-only.
	Cols   uint16 `json:"cols,omitempty"`
	Rows   uint16 `json:"rows,omitempty"`
	Buffer string `json:"buffer,omitempty"`

	// Editor-only.
	DocID string `json:"docId,omitempty"`
}

// webWindow is the runtime form of a window. PTY/buffer/cwd/agent only apply
// to shell windows; editor windows are pure layout state.
type webWindow struct {
	windowState
	pty    *os.File
	pid    int
	buffer []byte

	cwd           string
	agentKind     string
	agentSession  string
	historyOffset int
}

type webClient struct {
	id            int
	name          string
	x             int
	y             int
	authenticated bool
	conn          *websocket.Conn
	send          chan wsEnvelope
	out           chan wsBinOut
	hub           *webHub
}

// wsBinOut carries one terminal-output payload destined for a single client.
// Channeling raw bytes lets the hot path skip JSON marshaling and parse costs.
type wsBinOut struct {
	id   int64
	data []byte
}

// WebSocket binary frame layout (big-endian, versioned for future message
// types such as image output):
//
//	offset 0            : byte   version/category (1 = terminal output)
//	offset 1..8         : int64  window id
//	offset 9..          : payload bytes
const wsBurstCode = uint8(1)

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
	windows       map[int64]*webWindow
	idSeq         int64
	topZ          int
	password      string
	collabClients map[*collabClient]bool
	collabUpdates [][]byte

	persistDir  string
	persist     bool
	saveHistory bool
	dirty       bool
	snapMu      sync.Mutex
}

func newWebHub(password, persistDir string, persistEnabled, saveHistory bool) *webHub {
	return &webHub{
		clients:       make(map[int]*webClient),
		windows:       make(map[int64]*webWindow),
		password:      password,
		collabClients: make(map[*collabClient]bool),
		persistDir:    persistDir,
		persist:       persistEnabled,
		saveHistory:   saveHistory,
	}
}

// snapshotLocked returns the user list and all windows sorted by ID. The
// caller must hold h.mu.
//
// Terminal scrollback is only attached when includeBuffers is true — used for
// the one-time `hello` handshake a fresh client needs in order to replay the
// screen. Routine `state` broadcasts omit it: re-serializing up to 1 MiB of
// scrollback on every create/move/resize would otherwise flood the socket.
func (h *webHub) snapshotLocked(includeBuffers bool) (users []webUser, windows []windowState) {
	users = make([]webUser, 0, len(h.clients))
	windows = make([]windowState, 0, len(h.windows))
	for _, c := range h.clients {
		users = append(users, webUser{ID: c.id, Name: c.name, X: c.x, Y: c.y, Cursor: true})
	}
	for _, w := range h.windows {
		state := w.windowState
		if includeBuffers && w.Kind == persist.KindShell {
			state.Buffer = string(w.buffer)
		}
		windows = append(windows, state)
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].ID < windows[j].ID })
	return users, windows
}

func (h *webHub) markDirty() {
	h.mu.Lock()
	h.dirty = true
	h.mu.Unlock()
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

// broadcastOut fans one terminal-output payload out to every connected client
// as a binary frame, dropping any blocked reader rather than stalling the PTY.
func (h *webHub) broadcastOut(id int64, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.clients {
		select {
		case c.out <- wsBinOut{id: id, data: payload}:
		default:
		}
	}
}

func (h *webHub) broadcastState() {
	h.mu.Lock()
	// Routine state updates carry geometry/z-order/user list only. Terminal
	// scrollback is handled by the one-time `hello` handshake; including it
	// here on every create/move/resize would re-ship the full buffer to every
	// client repeatedly.
	users, windows := h.snapshotLocked(false)
	h.mu.Unlock()
	h.broadcast(wsEnvelope{Type: "state", Users: users, Windows: windows})
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
		out:           make(chan wsBinOut, 64),
		hub:           h,
	}
	if client.authenticated {
		h.clients[client.id] = client
	}
	users, windows := h.snapshotLocked(true)
	h.mu.Unlock()
	if client.authenticated {
		client.send <- wsEnvelope{Type: "hello", ID: int64(client.id), Users: users, Windows: windows}
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
		close(c.out)
		removed = true
	}
	h.mu.Unlock()
	if removed {
		h.broadcastState()
	}
}

// createShell starts a new shell window running command (or the default shell
// when command is empty) in cwd, and registers it with the hub.
func (h *webHub) createShell(x, y int, cols, rows, width, height, zIndex int, cwd string, command []string) (*webWindow, error) {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	if width == 0 {
		width = 760
	}
	if height == 0 {
		height = 420
	}
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}

	var cmd *exec.Cmd
	if len(command) > 0 {
		cmd = exec.Command(command[0], command[1:]...)
	} else {
		cmd = exec.Command(defaultShell())
	}
	cmd.Dir = cwd
	cmd.Env = terminalEnv("xterm-256color")
	p, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	h.idSeq++
	h.topZ++
	if zIndex <= 0 {
		zIndex = h.topZ
	}
	if zIndex > h.topZ {
		h.topZ = zIndex
	}
	win := &webWindow{
		windowState: windowState{
			ID: h.idSeq, Kind: persist.KindShell,
			X: x, Y: y, Width: width, Height: height, ZIndex: zIndex,
			Cols: uint16(cols), Rows: uint16(rows),
		},
		pty: p,
		pid: cmd.Process.Pid,
		cwd: cwd,
	}
	h.windows[win.ID] = win
	h.dirty = true
	h.mu.Unlock()

	go h.readShell(win)
	h.broadcastState()
	return win, nil
}

func (h *webHub) readShell(win *webWindow) {
	buf := make([]byte, 32*1024)
	for {
		n, err := win.pty.Read(buf)
		if n > 0 {
			// Take one copy: it feeds both the scrollback buffer (shared under
			// the hub lock) and the client fan-out. Because every client's
			// writer goroutine reads the same chunk concurrently, it must stay
			// immutable — never reuse the live pty read buffer after `Read`.
			chunk := append([]byte(nil), buf[:n]...)
			// Buffer a copy for scrollback under the lock.
			h.mu.Lock()
			win.buffer = append(win.buffer, chunk...)
			if len(win.buffer) > 1<<20 {
				win.buffer = win.buffer[len(win.buffer)-(1<<20):]
			}
			h.dirty = true
			h.mu.Unlock()
			// Send the raw bytes straight to clients as a binary frame,
			// skipping JSON marshaling (escaping/quotes) entirely.
			h.broadcastOut(win.ID, chunk)
		}
		if err != nil {
			h.closeWindow(win.ID)
			return
		}
	}
}

// createEditorWindow registers a new editor window. The ID is assigned by the
// hub so shell and editor windows share one ID space.
func (h *webHub) createEditorWindow(x, y, width, height int, docID string) {
	if width == 0 {
		width = 980
	}
	if height == 0 {
		height = 620
	}
	if docID == "" {
		docID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
	}

	h.mu.Lock()
	h.idSeq++
	h.topZ++
	win := &webWindow{
		windowState: windowState{
			ID: h.idSeq, Kind: persist.KindEditor, DocID: docID,
			X: x, Y: y, Width: width, Height: height, ZIndex: h.topZ,
		},
	}
	h.windows[win.ID] = win
	h.dirty = true
	h.mu.Unlock()
	h.broadcastState()
}

// patchWindow applies a geometry/z-order/size patch to any window. For shells
// with a terminal size change it also resizes the PTY.
func (h *webHub) patchWindow(id int64, patch windowPatch) {
	h.mu.Lock()
	win, ok := h.windows[id]
	if ok {
		if patch.X != nil {
			win.X = *patch.X
		}
		if patch.Y != nil {
			win.Y = *patch.Y
		}
		if patch.Width != nil {
			win.Width = *patch.Width
		}
		if patch.Height != nil {
			win.Height = *patch.Height
		}
		if patch.ZIndex != nil {
			win.ZIndex = *patch.ZIndex
			if *patch.ZIndex > h.topZ {
				h.topZ = *patch.ZIndex
			}
		}
		if win.Kind == persist.KindShell {
			if patch.Cols != nil {
				win.Cols = *patch.Cols
			}
			if patch.Rows != nil {
				win.Rows = *patch.Rows
			}
		}
		h.dirty = true
	}
	h.mu.Unlock()

	if ok && win.Kind == persist.KindShell && (patch.Cols != nil || patch.Rows != nil) {
		_ = pty.Setsize(win.pty, &pty.Winsize{Cols: win.Cols, Rows: win.Rows})
	}
	if ok {
		h.broadcastState()
	}
}

func (h *webHub) closeWindow(id int64) {
	h.mu.Lock()
	win, ok := h.windows[id]
	if ok {
		delete(h.windows, id)
		h.dirty = true
	}
	h.mu.Unlock()
	if ok {
		if win.pty != nil {
			_ = win.pty.Close()
		}
		if h.persist {
			_ = persist.DeleteHistory(h.persistDir, id)
		}
		h.broadcastState()
	}
}

func (h *webHub) window(id int64) *webWindow {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.windows[id]
}

const restoredMarker = "\r\n\x1b[2m── session restored after restart ──\x1b[0m\r\n"

// restore rebuilds the workspace from the on-disk snapshot. It returns true
// when at least one window was restored; the caller creates a default shell
// when it returns false.
func (h *webHub) restore() bool {
	if !h.persist {
		return false
	}
	snap, err := persist.Load(h.persistDir)
	if err != nil {
		log.Printf("failed to load session snapshot: %v", err)
		return false
	}
	if len(snap.Windows) == 0 {
		return false
	}

	// Reload the collaborative document (markdown content and drawings) so
	// editor windows come back with their contents, not just their frames.
	if updates, err := persist.LoadCollab(h.persistDir); err != nil {
		log.Printf("failed to load collab state: %v", err)
	} else if len(updates) > 0 {
		h.collabUpdates = updates
	}

	restored := false
	for _, saved := range snap.Windows {
		switch saved.Kind {
		case persist.KindEditor:
			h.mu.Lock()
			h.idSeq = saved.ID
			if saved.ZIndex > h.topZ {
				h.topZ = saved.ZIndex
			}
			win := &webWindow{windowState: windowState{
				ID: saved.ID, Kind: persist.KindEditor, DocID: saved.DocID,
				X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height, ZIndex: saved.ZIndex,
			}}
			h.windows[win.ID] = win
			h.mu.Unlock()
			restored = true
		case persist.KindShell:
			if err := h.restoreShell(saved); err != nil {
				log.Printf("failed to restore shell %d: %v", saved.ID, err)
				continue
			}
			restored = true
		}
	}
	if snap.IDSeq > h.idSeq {
		h.idSeq = snap.IDSeq
	}
	if restored {
		h.mu.Lock()
		shells, editors := 0, 0
		for _, w := range h.windows {
			if w.Kind == persist.KindShell {
				shells++
			} else {
				editors++
			}
		}
		h.mu.Unlock()
		log.Printf("restored %d window(s) from %s (%d shell, %d editor)", shells+editors, h.persistDir, shells, editors)
	}
	return restored
}

// restoreShell rebuilds one shell window: it resumes the agent session when one
// was recorded, otherwise starts a fresh shell in the saved working directory,
// then replays any saved scrollback into the buffer for visual continuity.
func (h *webHub) restoreShell(saved persist.Window) error {
	command := persist.ResumeCommand(saved.Agent)

	if cols := saved.Cols; cols == 0 {
		saved.Cols = 80
	}
	if rows := saved.Rows; rows == 0 {
		saved.Rows = 24
	}

	var cmd *exec.Cmd
	if len(command) > 0 {
		cmd = exec.Command(command[0], command[1:]...)
	} else {
		cmd = exec.Command(defaultShell())
	}
	cwd := saved.Cwd
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}
	cmd.Dir = cwd
	cmd.Env = terminalEnv("xterm-256color")
	p, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: saved.Cols, Rows: saved.Rows})
	if err != nil {
		return err
	}

	win := &webWindow{
		windowState: windowState{
			ID: saved.ID, Kind: persist.KindShell,
			X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height, ZIndex: saved.ZIndex,
			Cols: saved.Cols, Rows: saved.Rows,
		},
		pty: p,
		pid: cmd.Process.Pid,
		cwd: cwd,
	}

	// Replay saved scrollback so the pane shows its previous screen contents.
	// Replay whenever a history file exists, even if this run has
	// -persist-history off: the flag gates *writing* history, not reading back
	// what a previous run already saved.
	if history, err := persist.ReadHistory(h.persistDir, saved.ID); err == nil && len(history) > 0 {
		win.buffer = append(win.buffer, history...)
		win.buffer = append(win.buffer, restoredMarker...)
		win.historyOffset = len(win.buffer)
	}

	h.mu.Lock()
	h.windows[win.ID] = win
	if saved.ZIndex > h.topZ {
		h.topZ = saved.ZIndex
	}
	h.dirty = true
	h.mu.Unlock()

	go h.readShell(win)
	return nil
}

// snapshotLoop periodically persists the workspace when it has changed.
func (h *webHub) snapshotLoop(interval time.Duration) {
	if !h.persist {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := h.snapshot(); err != nil {
			log.Printf("failed to persist session: %v", err)
		}
	}
}

// snapshot writes the current window layout (and optional history) to disk if
// anything changed since the last snapshot.
func (h *webHub) snapshot() error {
	// Refresh agent/session and live cwd metadata first so this snapshot
	// captures them (the refresh itself may set dirty).
	h.refreshAgents()

	h.mu.Lock()
	if !h.dirty {
		h.mu.Unlock()
		return nil
	}
	h.dirty = false

	snap := &persist.Snapshot{IDSeq: h.idSeq, Windows: make([]persist.Window, 0, len(h.windows))}
	collabUpdates := make([][]byte, len(h.collabUpdates))
	for i, u := range h.collabUpdates {
		collabUpdates[i] = append([]byte(nil), u...)
	}
	type historyWrite struct {
		id   int64
		data []byte
	}
	var histories []historyWrite
	keep := make(map[int64]bool)

	for _, w := range h.windows {
		pw := persist.Window{
			ID: w.ID, Kind: w.Kind,
			X: w.X, Y: w.Y, Width: w.Width, Height: w.Height, ZIndex: w.ZIndex,
			DocID: w.DocID,
		}
		if w.Kind == persist.KindShell {
			pw.Cols, pw.Rows, pw.Cwd = w.Cols, w.Rows, w.cwd
			if w.agentKind != "" {
				pw.Agent = &persist.AgentRef{Kind: w.agentKind, SessionID: w.agentSession}
			}
			if h.saveHistory {
				keep[w.ID] = true
				if len(w.buffer) > w.historyOffset {
					histories = append(histories, historyWrite{id: w.ID, data: append([]byte(nil), w.buffer...)})
				}
			}
		}
		snap.Windows = append(snap.Windows, pw)
	}
	h.mu.Unlock()

	if err := persist.Write(h.persistDir, snap); err != nil {
		return err
	}
	if err := persist.WriteCollab(h.persistDir, collabUpdates); err != nil {
		return err
	}
	for _, hw := range histories {
		if err := persist.WriteHistory(h.persistDir, hw.id, hw.data); err != nil {
			return err
		}
		if win := h.window(hw.id); win != nil {
			h.mu.Lock()
			win.historyOffset = len(win.buffer)
			h.mu.Unlock()
		}
	}
	if h.saveHistory {
		return persist.PruneHistory(h.persistDir, keep)
	}
	return nil
}

// refreshAgents updates each shell window's agent/session metadata by probing
// its child processes, and tracks the live working directory (shells may cd
// after creation; an agent's own cwd determines which session to resume).
func (h *webHub) refreshAgents() {
	h.mu.Lock()
	type probe struct {
		id  int64
		pid int
	}
	var probes []probe
	for _, w := range h.windows {
		if w.Kind == persist.KindShell && w.pid > 0 {
			probes = append(probes, probe{id: w.ID, pid: w.pid})
		}
	}
	h.mu.Unlock()

	for _, pb := range probes {
		// Shell out outside the hub lock.
		ref, agentCwd := persist.DetectAgentForPID(pb.pid)
		shellCwd := ""
		if ref == nil {
			shellCwd = persist.ProcessCwd(pb.pid)
		}

		h.mu.Lock()
		if win, ok := h.windows[pb.id]; ok {
			if ref != nil {
				win.agentKind, win.agentSession = ref.Kind, ref.SessionID
				if agentCwd != "" && agentCwd != win.cwd {
					win.cwd = agentCwd
					h.dirty = true
				}
			} else {
				win.agentKind, win.agentSession = "", ""
				if shellCwd != "" && shellCwd != win.cwd {
					win.cwd = shellCwd
					h.dirty = true
				}
			}
		}
		h.mu.Unlock()
	}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },

	// Enable permessage-deflate so the socket negotiates per-message
	// compression with browsers that support it. Terminal sessions frequently
	// emit repeated whitespace/clear patterns, which compress extremely well;
	// the average latency/bandwidth win on slower links is large.
	EnableCompression: true,
}

// encodeBinOut frames one terminal-output payload as a WebSocket binary
// message. Layout: [version byte][int64 window id big-endian][payload bytes].
// Keeping output on binary frames skips JSON escaping/quotes on the hot path.
func encodeBinOut(id int64, payload []byte) []byte {
	buf := make([]byte, 9+len(payload))
	buf[0] = wsBurstCode
	binary.BigEndian.PutUint64(buf[1:], uint64(id))
	copy(buf[9:], payload)
	return buf
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
	ready, err := json.Marshal(wsEnvelope{Type: "ready", ID: int64(len(h.collabUpdates)), ClientID: client.clientID, Name: client.name, Color: client.color})
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
	h.dirty = true
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
			// One writer goroutine for both channels keeps frames ordered.
			// Select drains control (JSON) and terminal output (binary) fairly;
			// if `out` is full, we drop the oldest buffer so a slow client can
			// never stall a busy PTY. The client re-syncs via the next
			// "hello"/scrollback replay.
			for {
				select {
				case msg, ok := <-client.send:
					if !ok {
						return
					}
					if err := conn.WriteJSON(msg); err != nil {
						return
					}
				case out, ok := <-client.out:
					if !ok {
						return
					}
					if err := conn.WriteMessage(websocket.BinaryMessage, encodeBinOut(out.id, out.data)); err != nil {
						return
					}
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
				users, windows := hub.snapshotLocked(true)
				hub.mu.Unlock()
				client.send <- wsEnvelope{Type: "hello", ID: int64(client.id), Users: users, Windows: windows}
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
				if msg.Kind == persist.KindEditor {
					hub.createEditorWindow(msg.X, msg.Y, msg.Width, msg.Height, msg.DocID)
				} else {
					if _, err := hub.createShell(msg.X, msg.Y, int(msg.Cols), int(msg.Rows), msg.Width, msg.Height, 0, "", nil); err != nil {
						log.Printf("failed to create web shell: %v", err)
					}
				}
			case "input":
				if !client.authenticated {
					continue
				}
				if win := hub.window(msg.ID); win != nil && win.Kind == persist.KindShell && win.pty != nil {
					_, _ = win.pty.Write([]byte(msg.Data))
				}
			case "patch":
				if !client.authenticated {
					continue
				}
				if msg.Patch != nil {
					hub.patchWindow(msg.ID, *msg.Patch)
				}
			case "close":
				if !client.authenticated {
					continue
				}
				hub.closeWindow(msg.ID)
			}
		}
	}
}

// listenAddress combines a host/address and TCP port into the address accepted
// by net.Listen. net.JoinHostPort also handles IPv6 literals correctly.
func listenAddress(address string, port int) string {
	return net.JoinHostPort(address, strconv.Itoa(port))
}

// updateCommand runs `sshit upgrade` and returns its process exit code.
func updateCommand(args []string) int {
	return update.Run(version, args)
}

func main() {
	// "upgrade" is a CLI subcommand, not a service flag. Handle it before the
	// flag package gets a chance to treat it (or its flags) as server options.
	if len(os.Args) > 1 && os.Args[1] == "upgrade" {
		os.Exit(updateCommand(os.Args[2:]))
	}

	address := flag.String("address", "0.0.0.0", "address to listen on")
	flag.StringVar(address, "a", "0.0.0.0", "address to listen on")
	port := flag.Int("port", 2222, "port to listen on")
	flag.IntVar(port, "p", 2222, "port to listen on")
	password := flag.String("password", "", "password required for SSH and Web UI access")
	persist := flag.Bool("persist", true, "persist the workspace to ~/.sshit/<port>/ and restore it after a restart")
	persistHistory := flag.Bool("persist-history", true, "also save terminal scrollback to ~/.sshit/<port>/history/ and replay it after a restart (may contain secrets; disable with -persist-history=false)")
	configureDebugFlags()
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

	home, herr := os.UserHomeDir()
	if herr != nil {
		home = os.Getenv("HOME")
	}
	persistDir := filepath.Join(home, ".sshit", strconv.Itoa(*port))

	hub := newWebHub(*password, persistDir, *persist, *persistHistory)
	if *persist {
		log.Printf("session persistence enabled: %s (history replay %v)", persistDir, *persistHistory)
	}
	if !hub.restore() {
		if _, err := hub.createShell(0, 0, 80, 24, 760, 420, 0, "", nil); err != nil {
			log.Printf("failed to create initial web shell: %v", err)
		}
	}
	go hub.snapshotLoop(2 * time.Second)

	httpHandler, err := newHTTPHandler(hub)
	if err != nil {
		log.Fatalf("failed to load web UI: %v", err)
	}
	httpServer := &http.Server{Handler: httpHandler}

	addr := listenAddress(*address, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		listener = tcpKeepAliveListener{tcpListener}
	}

	log.Printf("using host key %s", hostKeyPath)
	logDebugMode()
	log.Printf("listening on %s for SSH and HTTP", addr)
	log.Printf("http://localhost:%d", *port)
	log.Fatal(serveMux(listener, sshServer, httpServer))
}
