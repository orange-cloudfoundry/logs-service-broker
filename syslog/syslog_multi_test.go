package syslog_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"

	"github.com/orange-cloudfoundry/logs-service-broker/syslog"
)

var _ = Describe("SyslogMulti", func() {
	var server1 *Server
	var server2 *Server
	var syslogClient io.WriteCloser
	BeforeEach(func() {
		var err error
		server1 = NewServer("tcp")
		server2 = NewServer("tcp")
		syslogClient, err = syslog.NewWriter(server1.URL, server2.URL)
		Expect(err).ToNot(HaveOccurred())

	})
	AfterEach(func() {
		syslogClient.Close()
		server1.Close()
		server2.Close()
	})
	It("should pass to all servers the content", func() {
		syslogClient.Write([]byte("my content"))

		Eventually(server1.BufferResp.String).Should(Equal("my content"))
		Eventually(server2.BufferResp.String).Should(Equal("my content"))
	})
})
