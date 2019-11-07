package syslog

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
)

type Writer struct {
	hostname string
	network  string
	raddr    string
	conn     serverConn
	mu       sync.Mutex // guards conn
}

type serverConn interface {
	writeString(string) error
	close() error
}

type netConn struct {
	local bool
	conn  net.Conn
}

func NewWriter(addresses ...string) (io.WriteCloser, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("One address must be given")
	}

	if len(addresses) > 1 {
		return MultiDial(addresses...)
	}

	addr := addresses[0]
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "http" || u.Scheme == "https" {
		return HttpDial(addr)
	}

	return Dial(addr)
}

func Dial(addr string) (*Writer, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()

	w := &Writer{
		hostname: hostname,
		network:  u.Scheme,
		raddr:    u.Host,
	}

	err = w.connect()
	if err != nil {
		return nil, err
	}
	return w, err
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *Writer) connect() (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.close()
		w.conn = nil
	}

	var c net.Conn
	c, err = net.Dial(w.network, w.raddr)
	if err == nil {
		w.conn = &netConn{conn: c}
		if w.hostname == "" {
			w.hostname = c.LocalAddr().String()
		}
	}
	return
}

func (w *Writer) writeAndRetry(s string) (int, error) {
	if w.conn != nil {
		if n, err := w.write(s); err == nil {
			return n, err
		}
	}
	if err := w.connect(); err != nil {
		return 0, err
	}

	return w.write(s)
}

func (w *Writer) write(msg string) (int, error) {
	err := w.conn.writeString(msg)
	if err != nil {
		return 0, err
	}
	// Note: return the length of the input, not the number of
	// bytes printed by Fprintf, because this must behave like
	// an io.Writer.
	return len(msg), nil
}

// Write sends a log message to the syslog daemon.
func (w *Writer) Write(b []byte) (int, error) {
	return w.writeAndRetry(string(b))
}

// Write sends a log message to the syslog daemon.
func (w *Writer) WriteString(mes string) (int, error) {
	return w.writeAndRetry(mes)
}

// Close closes a connection to the syslog daemon.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.close()
		w.conn = nil
		return err
	}
	return nil
}

func (n *netConn) writeString(mes string) error {
	_, err := fmt.Fprint(n.conn, mes)
	return err
}

func (n *netConn) close() error {
	return n.conn.Close()
}
