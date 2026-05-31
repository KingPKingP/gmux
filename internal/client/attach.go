package client

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"

	"gmux/internal/ipc"
)

func Attach(session string) error {
	c, err := ipc.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := c.Request(ipc.Request{Type: ipc.ReqAttach, Session: session}); err != nil {
		return err
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	ui := &frameUI{
		status:  "[gmux] starting...",
		session: session,
	}
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	ui.resize(w, h)
	_ = c.Send(ipc.Request{Type: ipc.ReqResize, Session: session, Cols: w, Rows: max(1, h-2)})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	defer signal.Stop(sig)
	go func() {
		for range sig {
			w, h, _ := term.GetSize(int(os.Stdout.Fd()))
			ui.resize(w, h)
			ui.drawBars()
			_ = c.Send(ipc.Request{Type: ipc.ReqResize, Session: session, Cols: w, Rows: max(1, h-2)})
		}
	}()

	done := make(chan error, 2)
	go readServer(c, ui, done)
	go readInput(c, session, done)

	fmt.Print("\x1b[?25l\x1b[?1049h\x1b[2J\x1b[H")
	ui.drawBars()
	defer fmt.Print("\x1b[r\x1b[?1049l\x1b[?25h")
	return <-done
}

type frameUI struct {
	mu      sync.Mutex
	width   int
	height  int
	status  string
	session string
}

func (f *frameUI) resize(w, h int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.width = w
	f.height = h
}

func (f *frameUI) setStatus(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = s
}

func (f *frameUI) drawBars() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.width <= 0 || f.height <= 2 {
		return
	}
	top := fmt.Sprintf(" GOMUX | session: %s ", f.session)
	bot := " " + f.status + " "
	top = padTrim(top, f.width)
	bot = padTrim(bot, f.width)
	fmt.Printf("\x1b[1;1H\x1b[7m%s\x1b[0m", top)
	fmt.Printf("\x1b[%d;1H\x1b[7m%s\x1b[0m", f.height, bot)
	fmt.Printf("\x1b[2;%dr", f.height-1)
	fmt.Print("\x1b[2;1H")
}

func readServer(c *ipc.Conn, ui *frameUI, done chan<- error) {
	for {
		resp, err := c.Recv()
		if err != nil {
			done <- err
			return
		}
		switch resp.Type {
		case ipc.EvData:
			_, _ = os.Stdout.Write(resp.Data)
		case ipc.EvInfo:
			ui.setStatus(resp.Info)
			ui.drawBars()
		case ipc.EvError:
			done <- fmt.Errorf(resp.Error)
			return
		}
	}
}

func padTrim(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	if len(r) < width {
		return s + strings.Repeat(" ", width-len(r))
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func readInput(c *ipc.Conn, session string, done chan<- error) {
	buf := make([]byte, 1)
	fd := int(os.Stdin.Fd())

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			done <- err
			return
		}
		if n == 0 {
			continue
		}
		b := buf[0]

		if b == 0x1d || b == 0x11 { // Ctrl-] or Ctrl-q
			done <- nil
			return
		}

		if b == 0x1b {
			nb, ok, rerr := readByteWithTimeout(fd, 50*time.Millisecond)
			if rerr != nil {
				done <- rerr
				return
			}
			if !ok {
				_ = c.Send(ipc.Request{Type: ipc.ReqInput, Session: session, Data: []byte{0x1b}})
				continue
			}
			switch {
			case nb >= '1' && nb <= '9':
				_ = c.Send(ipc.Request{Type: ipc.ReqTabSwitch, Session: session, Tab: int(nb - '0')})
			case nb == 'n' || nb == 'N':
				_ = c.Send(ipc.Request{Type: ipc.ReqTabNew, Session: session})
			default:
				_ = c.Send(ipc.Request{Type: ipc.ReqInput, Session: session, Data: []byte{0x1b, nb}})
			}
			continue
		}

		if err := c.Send(ipc.Request{Type: ipc.ReqInput, Session: session, Data: []byte{b}}); err != nil {
			done <- err
			return
		}
	}
}

func readByteWithTimeout(fd int, timeout time.Duration) (byte, bool, error) {
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	var set syscall.FdSet
	set.Bits[fd/64] |= 1 << (uint(fd) % 64)
	n, err := syscall.Select(fd+1, &set, nil, nil, &tv)
	if err != nil {
		return 0, false, err
	}
	if n == 0 {
		return 0, false, nil
	}
	var b [1]byte
	rn, rerr := os.Stdin.Read(b[:])
	if rerr != nil {
		return 0, false, rerr
	}
	if rn == 0 {
		return 0, false, nil
	}
	return b[0], true, nil
}
