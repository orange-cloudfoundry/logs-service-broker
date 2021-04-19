package syslog_test

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var TLSPrivKey = `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQDRH06eQO5W744o
WdbodkG4rFey0NQHFdWXV5igltEtnoglHi8ygNbNwIFklYJSBaUf5Dn9ZZSD1Ay0
vAOBj2iKmIoAsts77wuO11auD2fFBt2a2VbEGdWQGBcqfIKottaGJYf7ifoNcRwz
TgXngmnkZWMIGi9BnmmNtd5H9vfA7D/UKfJeTh/+JwbIbmmkLgm5YLsz2Xjm0YPH
isZqjSdvrvMJzOF3pypzxegr+JEVDvOozmIYvTyyQoMla3OTcWN6iu5hCpGdDlcq
q2ebYZGrC10GI/yXiabU+g/xpL/GaK2sy3GN+YIDtFxsBJjROP4oSB25YCZ1FhCm
zE7/I5aDAgMBAAECggEBAK1wO7IAxCuSDuLkb9roiWVyemGx1MfzkewtGEbIDsC5
JM00FYzbUkvfBvG3FhiU2fhzPq0snFohelBDRt0jZV7dWEdwD2fLwFg9vIQr/rJo
GU8eRlnp2zfg4wW3sl3fFli3s+oo6xxO3UanxTnW7aAhflrv7JWNnpmLZslkyOJV
DVFP1+gfGQJJLLGZdPYndfvwhU+hMymTo/RWS8sGDB6l6Tiezg/iipfb03msPpdL
RQIAl01+EUQcKkow2iG0/Z6gbFybYuSpZtb+6jbLBvG0nHqW8ZuK0z9E6/F2Ol1M
+af7uo14vi+n74yp6OZdBbbQljNF7e4TrfqDakrFIgECgYEA+XCXnKjhxIJJoEQS
XmDGi2Wkp8j0gROhVVf5kPxmpY3cpqe4CS5AeBCBz5J8miXeSQCbNUUXoxWWm1jZ
rQUzEpW+7mOOtf6+V1sM2+VEsqAEdpTHngb90VlXJaq1G9WQFgiWjlO3ZicCZqkM
JGfdVJYqaES2mIrQ/yy03HJfFIECgYEA1p9EteJhAFKWM5cfePU38SgLKHN7mGFu
iGTzM5i83n5eKcNb3c2t+UKKNdTSCArw+A/4M5IlIVSkOfwMhchu5/gbbCbui6Kg
FpE0kW43LorJ8bjUvkHjsotziaD853kbBnNb2FeVzgP7+X0uqtZkTMccKypSOlCi
xoXxyJkl2QMCgYEAnRmoo2ZKKzXToTi+SOqyoYD23yXVuKXgapvp9sLA82wRmHTx
l/ala/kZiN490+gdw+S53CcT6AbkwBqJnks0C3R8uC/D5iP3RZV219fiGI5nwTeb
MZA9s+iM1pBZWJp9ESN/j0xyqcfP31CA8TzpTSj2tIzyY8iqMMy7bEwsTgECgYAB
u31hfndL+l6uAe1GG6yc7LbSV8RKoZaz0STJaNU1co2uBp6qNqvN1ESrVJFxcS0q
w248dFSKZVWCBk/PkKOcibsm71WDmQdzxy5Gcj5NyN8CbXyCIKQG3+tJ1BvWfnrC
XZIDOAnEhPG2vNTwmhRrLjxC+O96+wWlVpVyChJtIwKBgQCDAQM5CFseWiBoMnPK
AW6wsdjy8E7SQDuW6HyFMAKj4mYVb76q927MO4v23m93a4A/pbcIkjfadUIYfMJY
cjlBVsyfZ/r6JTIbJgDyveYcD9QeQs5NRWQTcD8WvA7igf+9aLmj7PWRnDElJgqt
/fB3FYb9aJrRyGJcUbvNovEooQ==
-----END PRIVATE KEY-----
`

var TLSPubKey = `-----BEGIN CERTIFICATE-----
MIIDijCCAnICCQD9DAEb+F/INDANBgkqhkiG9w0BAQsFADCBhjELMAkGA1UEBhMC
REUxDDAKBgNVBAgMA05SVzEOMAwGA1UEBwwFRWFydGgxFzAVBgNVBAoMDlJhbmRv
bSBDb21wYW55MQswCQYDVQQLDAJJVDEXMBUGA1UEAwwOd3d3LnJhbmRvbS5jb20x
GjAYBgkqhkiG9w0BCQEWC2Zvb0BiYXIuY29tMB4XDTIxMDIxNjE1MjI1MFoXDTMx
MDIxNDE1MjI1MFowgYYxCzAJBgNVBAYTAkRFMQwwCgYDVQQIDANOUlcxDjAMBgNV
BAcMBUVhcnRoMRcwFQYDVQQKDA5SYW5kb20gQ29tcGFueTELMAkGA1UECwwCSVQx
FzAVBgNVBAMMDnd3dy5yYW5kb20uY29tMRowGAYJKoZIhvcNAQkBFgtmb29AYmFy
LmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANEfTp5A7lbvjihZ
1uh2QbisV7LQ1AcV1ZdXmKCW0S2eiCUeLzKA1s3AgWSVglIFpR/kOf1llIPUDLS8
A4GPaIqYigCy2zvvC47XVq4PZ8UG3ZrZVsQZ1ZAYFyp8gqi21oYlh/uJ+g1xHDNO
BeeCaeRlYwgaL0GeaY213kf298DsP9Qp8l5OH/4nBshuaaQuCblguzPZeObRg8eK
xmqNJ2+u8wnM4XenKnPF6Cv4kRUO86jOYhi9PLJCgyVrc5NxY3qK7mEKkZ0OVyqr
Z5thkasLXQYj/JeJptT6D/Gkv8ZorazLcY35ggO0XGwEmNE4/ihIHblgJnUWEKbM
Tv8jloMCAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAD6y3futjbvwUwYLxSyZNXcC4
dPau/Pal87AegphZSuA7JrDMN9fi0OHS/yt5+oMsjZNzSn5t+2v8WUoJNVkpzpHv
lafHTwTQFDTsqGwMvqJYZ+AV5XZSfr2K7wuRr+FDlam2nvUjipo7Sxdk3DPIwyC0
OGJNT1QlxhZgXmQ0ptq7zNSOJgaRHngVo7l4HY+/bHJ03JLozLotHduWmwdU/aWh
I6G9jGVBG1HmtpOsEYXSra3y2IpfB8QkhFWBjvUFS8PaGS86rj5J2Lvv2JGTxrTr
3KlVWt0xkTjguU/JmJUwyZhZfCqrDpFIxguz7mIhQR5kiU2tWszN6QMh8xftvw==
-----END CERTIFICATE-----
`

func TestSyslog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syslog Suite")
}

func NewServer(typeServer string) *Server {
	buf := &bytes.Buffer{}
	server := &Server{
		BufferResp: buf,
	}

	switch typeServer {
	case "tcp":
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic("Error listening: " + err.Error())
		}
		server.listener = l
		server.typeListener = "tcp"
	case "udp":
		l, err := net.Listen("udp", "127.0.0.1:0")
		if err != nil {
			panic("Error listening: " + err.Error())
		}
		server.listener = l
		server.typeListener = "udp"
	case "tcp+tls":
		cert, err := tls.X509KeyPair([]byte(TLSPubKey), []byte(TLSPrivKey))
		if err != nil {
			panic("server: loadkeys: " + err.Error())
		}
		config := tls.Config{Certificates: []tls.Certificate{cert}}
		config.Rand = rand.Reader
		l, err := tls.Listen("tcp", "127.0.0.1:0", &config)
		if err != nil {
			panic("server: listen: " + err.Error())
		}
		server.listener = l
		server.typeListener = "tcp+tls"
	default:
		server.httpServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			b, err := ioutil.ReadAll(request.Body)
			if err != nil {
				panic(err)
			}
			buf.Write(b)
		}))
	}
	server.Start()
	return server
}

type Server struct {
	URL          string
	BufferResp   *bytes.Buffer
	httpServer   *httptest.Server
	listener     net.Listener
	typeListener string
	isClosed     bool
}

func (s *Server) Start() {
	if s.httpServer != nil {
		s.httpServer.Start()
		s.URL = s.httpServer.URL
		return
	}
	go s.listenListener()
	s.URL = s.typeListener + "://" + s.listener.Addr().String()
}

func (s *Server) listenListener() {
	for {
		// Listen for an incoming connection.
		conn, err := s.listener.Accept()
		if err != nil {
			if s.isClosed {
				return
			}
			panic("Error accepting: " + err.Error())
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}

func (s *Server) handleRequest(conn net.Conn) {

	defer conn.Close()
	io.Copy(s.BufferResp, conn)
}

func (s *Server) Close() {
	s.isClosed = true
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
}
