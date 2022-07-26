package parser_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/go-loggregator/rpc/loggregator_v2"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/parser"
)

var _ = Describe("Parser", func() {
	var gParser *parser.Parser
	var appFilter parser.Filter
	var programPatterns = []string{
		`%{TIME} \|\-%{LOGLEVEL:@level} in %{NOTSPACE:[app][logger]} - %{GREEDYDATA:@message}`,
		`\[CONTAINER\]%{SPACE}%{NOTSPACE}%{SPACE}%{LOGLEVEL:@level}%{SPACE}%{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}%{HOSTNAME:[app][hostname]} - - \[%{HTTPDATE:[app][timestamp]}\] "%{WORD:[app][verb]} %{URIPATHPARAM:[app][path]} %{PROG:[app][http_spec]}" %{BASE10NUM:[app][status]:int} %{BASE10NUM:[app][request_bytes_received]:int} vcap_request_id=%{NOTSPACE:@request_id} %{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_ALT:[app][timestamp]}\] \[(core|mpm_event):%{WORD:@level}\] %{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_TXT:[app][timestamp]}\] %{LOGLEVEL:@level}: %{GREEDYDATA:@message}`,
	}
	var (
		org_id   = "c40e018a-c659-4280-887b-f0a4dd13d301"
		space_id = "a15de7be-92de-43fe-b3c1-850984392512"
		app_id   = "0012e905-7718-4ae4-9561-3e6d732db43c"
		app      = "my-app"
		msg      = "my message"
	)

	BeforeEach(func() {
		parsingKeys := []model.ParsingKey{
			{
				Name: "app.audit_data.messages.last",
				Hide: true,
			},
		}
		gParser = parser.NewParser(parsingKeys, true)
		for _, filter := range gParser.GetFilters() {
			if fmt.Sprintf("%T", filter) == "*parser.AppFilter" {
				appFilter = filter
			}
		}
	})

	Context("Filter App Log", func() {
		var FilterAppLog = func(ProcessID string) {
			logMessage := buildLogEnvelope(
				time.Now().UnixNano(),
				app_id,
				"1",
				msg,
				loggregator_v2.Log_OUT,
				nil)

			message, err := logMessage.Syslog(
				loggregator_v2.WithSyslogAppName(app),
				loggregator_v2.WithSyslogHostname(fmt.Sprintf("#{org}.#{space}.#{app}")),
				loggregator_v2.WithSyslogProcessID(ProcessID),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(message)).ToNot(BeZero())
			Expect(gParser).ToNot(BeNil())
			Expect(appFilter).ToNot(BeNil())

			var metadata = getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, message[0], programPatterns)
			Expect(err).ToNot(HaveOccurred())
			Expect(appFilter.Match(parsed)).To(BeTrue())
		}
		It("should filter app log", func() {
			FilterAppLog("[APP/PROC/WEB/0]")
			FilterAppLog("[APP/PROC/WEB/19]")
			FilterAppLog("[APP/PROC/WEB/SIDECAR/TOMCAT/0]")
			FilterAppLog("[APP/PROC/WEB/SIDECAR/TOMCAT/19]")
			FilterAppLog("[APP/PROC/WEB/SIDECAR/CONFIG-SERVER/0]")
			FilterAppLog("[APP/PROC/WEB/SIDECAR/CONFIG-SERVER/19]")
			FilterAppLog("[APP/TASK/MYTASK/0]")
			FilterAppLog("[APP/TASK/MYTASK/19]")
			FilterAppLog("[APP/TASK/MY-TASK/0]")
			FilterAppLog("[APP/TASK/MY-TASK/19]")
			FilterAppLog("[APP/TASK/bdfgr0d/0]")
		})
	})
})
