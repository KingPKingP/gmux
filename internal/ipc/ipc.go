package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	ReqSessionCreate = "session.create"
	ReqSessionList   = "session.list"
	ReqSessionKill   = "session.kill"
	ReqAttach        = "session.attach"
	ReqInput         = "pty.input"
	ReqResize        = "pty.resize"
	ReqTabSwitch     = "tab.switch"
	ReqTabNew        = "tab.new"
)

const (
	EvAck      = "ack"
	EvError    = "error"
	EvData     = "pty.data"
	EvInfo     = "info"
	EvDetached = "detached"
)

type Request struct {
	Type    string `json:"type"`
	Session string `json:"session,omitempty"`
	Data    []byte `json:"data,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Tab     int    `json:"tab,omitempty"`
}

type Response struct {
	Type     string   `json:"type"`
	OK       bool     `json:"ok,omitempty"`
	Error    string   `json:"error,omitempty"`
	Data     []byte   `json:"data,omitempty"`
	Sessions []string `json:"sessions,omitempty"`
	Info     string   `json:"info,omitempty"`
}

func SocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" {
		return filepath.Join(runtimeDir, "gmux.sock")
	}
	return fmt.Sprintf("/tmp/gmux-%d.sock", os.Getuid())
}

type Conn struct {
	netConn net.Conn
	enc     *json.Encoder
	dec     *json.Decoder
}

func Dial() (*Conn, error) {
	sock := SocketPath()
	c, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return nil, err
	}
	return &Conn{
		netConn: c,
		enc:     json.NewEncoder(c),
		dec:     json.NewDecoder(bufio.NewReader(c)),
	}, nil
}

func (c *Conn) Close() error {
	return c.netConn.Close()
}

func (c *Conn) Send(req Request) error {
	return c.enc.Encode(req)
}

func (c *Conn) Recv() (Response, error) {
	var r Response
	err := c.dec.Decode(&r)
	return r, err
}

func (c *Conn) Request(req Request) (Response, error) {
	if err := c.Send(req); err != nil {
		return Response{}, err
	}
	resp, err := c.Recv()
	if err != nil {
		return Response{}, err
	}
	if !resp.OK && resp.Type == EvError {
		return resp, fmt.Errorf(resp.Error)
	}
	return resp, nil
}
