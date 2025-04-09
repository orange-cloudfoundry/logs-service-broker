package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

type Writer struct {
	hostname           string
	network            string
	raddr              string
	conn               serverConn
	mu                 sync.Mutex // guards conn
	tlsConf            *tls.Config
	inTls              bool
	nbConnTry          int
	muTry              sync.Mutex
	hasBeenReconnected bool
}

type serverConn interface {
	writeString(string) error
	close() error
}

type netConn struct {
	conn net.Conn
}

func NewWriter(addresses ...string) (io.WriteCloser, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("one address must be given")
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

	inTls := false
	scheme := u.Scheme
	if u.Scheme == "tcp+tls" {
		inTls = true
		scheme = "tcp"
	}
	tlsConf, err := tlsConfigFromAddr(u)
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()

	w := &Writer{
		hostname: hostname,
		network:  scheme,
		raddr:    u.Host,
		tlsConf:  tlsConf,
		inTls:    inTls,
	}

	err = w.connect()
	if err != nil {
		return nil, err
	}
	return w, err
}

func tlsConfigFromAddr(u *url.URL) (*tls.Config, error) {
	tlsConf := &tls.Config{}

	verifyParam := u.Query().Get("verify")
	if verifyParam != "" {
		verify, err := strconv.ParseBool(verifyParam)
		if err != nil {
			verify = true
		}
		tlsConf.InsecureSkipVerify = !verify
	}

	certPath := u.Query().Get("cert")
	if certPath == "" {
		return tlsConf, nil
	}
	b, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(b)
	tlsConf.RootCAs = pool

	return tlsConf, nil
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *Writer) connect() (err error) {

	w.muTry.Lock()
	if w.nbConnTry >= 20 {
		w.muTry.Unlock()
		return fmt.Errorf("20 connect tries has been already made, erroring for preferring drop instead of server locking cause a lot of goroutine")
	}
	w.nbConnTry += 1
	w.muTry.Unlock()

	if w.nbConnTry > 0 && w.hasBeenReconnected {
		w.muTry.Lock()
		// if last connect try we remove
		if w.nbConnTry == 1 {
			w.hasBeenReconnected = false
		}
		w.nbConnTry -= 1
		w.muTry.Unlock()
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		err := w.conn.close()
		if err != nil {
			log.Printf("error closing connection: %v", err)
		}
		w.conn = nil
	}

	var c net.Conn
	if !w.inTls {
		c, err = net.DialTimeout(w.network, w.raddr, 5*time.Second)
	} else {
		c, err = tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, w.network, w.raddr, w.tlsConf)
	}
	if err != nil {
		w.muTry.Lock()
		w.nbConnTry -= 1
		w.muTry.Unlock()
		return err
	}

	w.conn = &netConn{conn: c}
	if w.hostname == "" {
		w.hostname = c.LocalAddr().String()
	}
	w.muTry.Lock()
	w.hasBeenReconnected = true
	w.nbConnTry -= 1
	w.muTry.Unlock()

	return nil
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

// WriteString sends a log message to the syslog daemon.
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
