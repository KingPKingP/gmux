package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"gmux/internal/ipc"
)

type tab struct {
	Index     int
	Name      string
	Cmd       *exec.Cmd
	PTY       *os.File
	Scroll    bytes.Buffer
	scrollCap int
	mu        sync.Mutex
}

type session struct {
	Name      string
	Tabs      map[int]*tab
	ActiveTab int
}

type clientAttach struct {
	session string
	conn    net.Conn
	state   *connState
}

type connState struct {
	enc *json.Encoder
	mu  sync.Mutex
}

func (s *connState) send(resp ipc.Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(resp)
}

type daemon struct {
	mu       sync.RWMutex
	sessions map[string]*session
	clients  map[net.Conn]*clientAttach
}

func RunForeground() error {
	sock := ipc.SocketPath()
	_ = os.Remove(sock)
	if err := os.MkdirAll(filepath.Dir(sock), 0o755); err != nil {
		return err
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		return err
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	}()

	d := &daemon{
		sessions: map[string]*session{},
		clients:  map[net.Conn]*clientAttach{},
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			continue
		}
		go d.handleConn(conn)
	}
}

func EnsureBackground() error {
	sock := ipc.SocketPath()
	if _, err := net.DialTimeout("unix", sock, 250*time.Millisecond); err == nil {
		return nil
	}
	cmd := exec.Command(os.Args[0], "server")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		if _, err := net.DialTimeout("unix", sock, 250*time.Millisecond); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not start")
}

func (d *daemon) handleConn(conn net.Conn) {
	defer func() {
		d.mu.Lock()
		delete(d.clients, conn)
		d.mu.Unlock()
		_ = conn.Close()
	}()

	dec := json.NewDecoder(conn)
	state := &connState{enc: json.NewEncoder(conn)}
	for {
		var req ipc.Request
		if err := dec.Decode(&req); err != nil {
			if err != io.EOF {
				_ = state.send(ipc.Response{Type: ipc.EvError, OK: false, Error: err.Error()})
			}
			return
		}
		if err := d.handleReq(conn, state, req); err != nil {
			_ = state.send(ipc.Response{Type: ipc.EvError, OK: false, Error: err.Error()})
		}
	}
}

func (d *daemon) handleReq(conn net.Conn, state *connState, req ipc.Request) error {
	switch req.Type {
	case ipc.ReqSessionCreate:
		if req.Session == "" {
			return errors.New("missing session name")
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		if _, exists := d.sessions[req.Session]; exists {
			return errors.New("session already exists")
		}
		s := &session{Name: req.Session, Tabs: map[int]*tab{}, ActiveTab: 1}
		t, err := spawnTab(1)
		if err != nil {
			return err
		}
		s.Tabs[1] = t
		d.sessions[req.Session] = s
		go d.readPTY(req.Session, 1, t)
		return state.send(ipc.Response{Type: ipc.EvAck, OK: true, Info: "created"})
	case ipc.ReqSessionList:
		d.mu.RLock()
		names := make([]string, 0, len(d.sessions))
		for k := range d.sessions {
			names = append(names, k)
		}
		d.mu.RUnlock()
		sort.Strings(names)
		return state.send(ipc.Response{Type: ipc.EvAck, OK: true, Sessions: names})
	case ipc.ReqSessionKill:
		if req.Session == "" {
			return errors.New("missing session name")
		}
		d.mu.Lock()
		s, ok := d.sessions[req.Session]
		if ok {
			for _, t := range s.Tabs {
				_ = t.PTY.Close()
				if t.Cmd.Process != nil {
					_ = t.Cmd.Process.Kill()
				}
			}
			delete(d.sessions, req.Session)
		}
		d.mu.Unlock()
		if !ok {
			return errors.New("session not found")
		}
		return state.send(ipc.Response{Type: ipc.EvAck, OK: true, Info: "killed"})
	case ipc.ReqAttach:
		d.mu.Lock()
		s, ok := d.sessions[req.Session]
		if !ok {
			d.mu.Unlock()
			return errors.New("session not found")
		}
		d.clients[conn] = &clientAttach{session: req.Session, conn: conn, state: state}
		active := s.ActiveTab
		t := s.Tabs[active]
		history := t.scrollSnapshot()
		d.mu.Unlock()
		_ = state.send(ipc.Response{Type: ipc.EvInfo, OK: true, Info: shortcuts(active, len(s.Tabs))})
		return state.send(ipc.Response{Type: ipc.EvData, OK: true, Data: history})
	case ipc.ReqInput:
		return d.writePTY(req.Session, req.Data)
	case ipc.ReqResize:
		return d.resizeActivePTY(req.Session, req.Cols, req.Rows)
	case ipc.ReqTabSwitch:
		return d.switchTab(req.Session, req.Tab, state)
	case ipc.ReqTabNew:
		return d.newTab(req.Session, state)
	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}
}

func spawnTab(index int) (*tab, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &tab{
		Index:     index,
		Name:      fmt.Sprintf("tab-%d", index),
		Cmd:       cmd,
		PTY:       ptmx,
		scrollCap: 1 << 20,
	}, nil
}

func (d *daemon) readPTY(sessionName string, tabIndex int, t *tab) {
	buf := make([]byte, 8192)
	for {
		n, err := t.PTY.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			t.appendScroll(chunk)
			d.broadcastActive(sessionName, tabIndex, chunk)
		}
		if err != nil {
			return
		}
	}
}

func (t *tab) appendScroll(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Scroll.Len()+len(data) > t.scrollCap {
		extra := t.Scroll.Len() + len(data) - t.scrollCap
		b := t.Scroll.Bytes()
		if extra < len(b) {
			t.Scroll.Reset()
			t.Scroll.Write(b[extra:])
		} else {
			t.Scroll.Reset()
		}
	}
	t.Scroll.Write(data)
}

func (t *tab) scrollSnapshot() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.Scroll.Bytes()...)
}

func (d *daemon) broadcastActive(sessionName string, tabIndex int, data []byte) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, cl := range d.clients {
		if cl.session != sessionName {
			continue
		}
		s := d.sessions[sessionName]
		if s == nil || s.ActiveTab != tabIndex {
			continue
		}
		_ = cl.state.send(ipc.Response{Type: ipc.EvData, OK: true, Data: data})
	}
}

func (d *daemon) writePTY(sessionName string, data []byte) error {
	d.mu.RLock()
	s := d.sessions[sessionName]
	d.mu.RUnlock()
	if s == nil {
		return errors.New("session not found")
	}
	t := s.Tabs[s.ActiveTab]
	if t == nil {
		return errors.New("active tab missing")
	}
	_, err := t.PTY.Write(data)
	return err
}

func (d *daemon) resizeActivePTY(sessionName string, cols, rows int) error {
	d.mu.RLock()
	s := d.sessions[sessionName]
	d.mu.RUnlock()
	if s == nil {
		return errors.New("session not found")
	}
	t := s.Tabs[s.ActiveTab]
	if t == nil {
		return errors.New("active tab missing")
	}
	return pty.Setsize(t.PTY, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (d *daemon) switchTab(sessionName string, tabNum int, state *connState) error {
	d.mu.Lock()
	s := d.sessions[sessionName]
	if s == nil {
		d.mu.Unlock()
		return errors.New("session not found")
	}
	t, ok := s.Tabs[tabNum]
	if !ok {
		d.mu.Unlock()
		return errors.New("tab not found")
	}
	s.ActiveTab = tabNum
	history := t.scrollSnapshot()
	tabsCount := len(s.Tabs)
	d.mu.Unlock()
	if err := state.send(ipc.Response{Type: ipc.EvInfo, OK: true, Info: shortcuts(tabNum, tabsCount)}); err != nil {
		return err
	}
	return state.send(ipc.Response{Type: ipc.EvData, OK: true, Data: append([]byte("\x1b[2J\x1b[H"), history...)})
}

func (d *daemon) newTab(sessionName string, state *connState) error {
	d.mu.Lock()
	s := d.sessions[sessionName]
	if s == nil {
		d.mu.Unlock()
		return errors.New("session not found")
	}
	next := 1
	for {
		if _, ok := s.Tabs[next]; !ok {
			break
		}
		next++
	}
	t, err := spawnTab(next)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	s.Tabs[next] = t
	s.ActiveTab = next
	tabCount := len(s.Tabs)
	d.mu.Unlock()
	go d.readPTY(sessionName, next, t)
	_ = state.send(ipc.Response{Type: ipc.EvInfo, OK: true, Info: shortcuts(next, tabCount)})
	return state.send(ipc.Response{Type: ipc.EvData, OK: true, Data: []byte("\x1b[2J\x1b[H")})
}

func shortcuts(active int, tabCount int) string {
	keys := []string{"detach Ctrl-]/Ctrl-q", "new-tab Alt+n", "switch Alt+1..9"}
	return fmt.Sprintf("[gmux] tabs=%d active=%d | %s", tabCount, active, strings.Join(keys, " | "))
}
