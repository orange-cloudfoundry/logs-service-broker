package syslog_test

import (
	"bytes"
	"compress/gzip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/orange-cloudfoundry/logs-service-broker/syslog"
	"io"
)

var _ = Describe("SyslogRaw", func() {
	Context("TCP/UDP server", func() {
		var server *Server
		var syslogClient io.WriteCloser
		BeforeEach(func() {
			var err error
			server = NewServer("tcp")
			syslogClient, err = syslog.NewWriter(server.URL)
			Expect(err).ToNot(HaveOccurred())

		})
		AfterEach(func() {
			syslogClient.Close()
			server.Close()
		})
		It("should pass to server the content", func() {
			// nolint:errcheck
			syslogClient.Write([]byte("my content"))
			Eventually(server.BufferResp.String).Should(Equal("my content"))
		})
	})

	Context("HTTP server", func() {
		var server *Server
		var syslogClient io.WriteCloser
		BeforeEach(func() {
			var err error
			server = NewServer("http")
			syslogClient, err = syslog.HttpDial(server.URL)
			Expect(err).ToNot(HaveOccurred())

		})
		AfterEach(func() {
			// nolint:errcheck
			syslogClient.Close()
			server.Close()
		})
		It("should pass to server the content", func() {
			// nolint:errcheck
			syslogClient.Write([]byte("my content"))
			Eventually(server.BufferResp.String).Should(Equal("my content"))
		})
		When("set in gzip", func() {
			It("should pass to server the content in gzip format", func() {
				var err error
				syslogClient, err = syslog.HttpDial(server.URL + "?in_gzip=true")
				Expect(err).ToNot(HaveOccurred())

				content := []byte("my content")
				// nolint:errcheck
				syslogClient.Write(content)

				buf := &bytes.Buffer{}
				gw := gzip.NewWriter(buf)
				// nolint:errcheck
				gw.Write(content)
				gw.Flush()
				gw.Close()

				Eventually(server.BufferResp.Bytes).Should(Equal(buf.Bytes()))
			})
		})
	})

	Context("TCP Tls server", func() {
		var server *Server
		var syslogClient io.WriteCloser
		BeforeEach(func() {
			var err error
			server = NewServer("tcp+tls")
			syslogClient, err = syslog.NewWriter(server.URL + "?verify=false")
			Expect(err).ToNot(HaveOccurred())

		})
		AfterEach(func() {
			syslogClient.Close()
			server.Close()
		})
		It("should pass to server the content", func() {
			// nolint:errcheck
			syslogClient.Write([]byte("my content"))
			Eventually(server.BufferResp.String).Should(Equal("my content"))
		})
	})
})
