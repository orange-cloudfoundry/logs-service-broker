package parser_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"

	"github.com/cloudfoundry/go-loggregator/rpc/loggregator_v2"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/parser"
)

var _ = Describe("Parser", func() {
	var gParser *parser.Parser
	var programPatterns = []string{
		`%{TIME} \|\-%{LOGLEVEL:@level} in %{NOTSPACE:[app][logger]} - %{GREEDYDATA:@message}`,
		`\[CONTAINER\]%{SPACE}%{NOTSPACE}%{SPACE}%{LOGLEVEL:@level}%{SPACE}%{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}%{HOSTNAME:[app][hostname]} - - \[%{HTTPDATE:[app][timestamp]}\] "%{WORD:[app][verb]} %{URIPATHPARAM:[app][path]} %{PROG:[app][http_spec]}" %{BASE10NUM:[app][status]:int} %{BASE10NUM:[app][request_bytes_received]:int} vcap_request_id=%{NOTSPACE:@request_id} %{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_ALT:[app][timestamp]}\] \[(core|mpm_event):%{WORD:@level}\] %{GREEDYDATA:@message}`,
		`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_TXT:[app][timestamp]}\] %{LOGLEVEL:@level}: %{GREEDYDATA:@message}`,
	}
	var (
		org_id   = "c40e018a-c659-4280-887b-f0a4dd13d301"
		org      = "my-org"
		space_id = "a15de7be-92de-43fe-b3c1-850984392512"
		space    = "my-space"
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
	})

	Context("Parse App Log", func() {

		var ParseTaskLog = func(processID string, taskID float64) {
			logMessage := buildLogEnvelope(
				time.Now().UnixNano(),
				app_id,
				"1",
				msg,
				loggregator_v2.Log_OUT,
				nil,
			)

			message, err := logMessage.Syslog(
				loggregator_v2.WithSyslogAppName(app),
				loggregator_v2.WithSyslogHostname(fmt.Sprintf("%s.%s.%s", org, space, app)),
				loggregator_v2.WithSyslogProcessID(processID),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(message)).ToNot(BeZero())
			Expect(gParser).ToNot(BeNil())

			var metadata = getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, message[0], programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["task_id"]).To(Equal(taskID))

			Expect(jsonLog["@message"]).To(Equal(fmt.Sprintln(msg)))
			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("APP"))
		}

		var ParseProcLog = func(processID string, instanceID float64) {
			logMessage := buildLogEnvelope(
				time.Now().UnixNano(),
				app_id,
				"1",
				msg,
				loggregator_v2.Log_OUT,
				nil,
			)

			message, err := logMessage.Syslog(
				loggregator_v2.WithSyslogAppName(app),
				loggregator_v2.WithSyslogHostname(fmt.Sprintf("%s.%s.%s", org, space, app)),
				loggregator_v2.WithSyslogProcessID(processID),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(message)).ToNot(BeZero())
			Expect(gParser).ToNot(BeNil())

			var metadata = getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, message[0], programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["app_instance"]).To(Equal(instanceID))

			Expect(jsonLog["@message"]).To(Equal(fmt.Sprintln(msg)))
			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("APP"))
		}

		It("returns expected @cf fields", func() {
			ParseProcLog("[APP/PROC/WEB/0]", float64(0))
			//ParseAppLog("[APP/PROC/WEB/19]", float64(19))
			ParseProcLog("[APP/PROC/WEB/SIDECAR/TOMCAT/0]", 0)
			ParseProcLog("[APP/PROC/WEB/SIDECAR/TOMCAT/19]", 19)
			ParseProcLog("[APP/PROC/WEB/SIDECAR/CONFIG-SERVER/0]", 0)
			ParseProcLog("[APP/PROC/WEB/SIDECAR/CONFIG-SERVER/19]", 19)
			ParseTaskLog("[APP/TASK/MYTASK/0]", float64(0))
			ParseTaskLog("[APP/TASK/MYTASK/19]", float64(19))
			ParseTaskLog("[APP/TASK/MY-TASK/0]", float64(0))
			ParseTaskLog("[APP/TASK/MY-TASK/19]", 19)
			ParseTaskLog("[APP/TASK/bdfgr0d/0]", 0)
		})
	})

	Context("Parse RTR Log", func() {

		It("returns expected fields", func() {
			currentTime := time.Now()
			timestamp := currentTime.Format("2006-01-02T15:04:05.999999Z")
			rtrTpl := `<14>1 %s %s.%s.%s - [RTR/3] - - %s.hbx.geo.francetelecom.fr:443 - [%s] "GET /v0/contracts?context=care&fields=all&oidval=0x71245ce0e8a200010001658f&type=os HTTP/1.1" 200 0 2 "-" "Mozilla/5.0 (Linux; Android 8.1.0; DUB-AL20) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.85 Mobile Safari/537.36" "10.77.106.3:35587" "10.77.106.122:61104" x_forwarded_for:"10.117.28.10, 10.77.106.3" x_forwarded_proto:"https" vcap_request_id:"f7314b39-3a9c-45e5-78fc-ae4b1737d4fd" response_time:0.055384 gorouter_time:0.000252 app_id:"%s" app_index:"10" x_cf_routererror:"-" x_b3_traceid:"9a1882fe065a9d9f" x_b3_spanid:"9a1882fe065a9d9f" x_b3_parentspanid:"-" b3:"9a1882fe065a9d9f-9a1882fe065a9d9f"`
			rtrMsg := fmt.Sprintf(rtrTpl, timestamp, org, space, app, app, timestamp, app_id)
			metadata := getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, []byte(rtrMsg), programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["app_instance"]).To(Equal(float64(3)))

			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("RTR"))
			Expect(jsonLog["rtr"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["rtr"].(map[string]interface{})["app_index"]).To(Equal(float64(10)))
			Expect(jsonLog["rtr"].(map[string]interface{})["hostname"]).To(Equal(fmt.Sprintf("%s.hbx.geo.francetelecom.fr", app)))
			Expect(jsonLog["rtr"].(map[string]interface{})["verb"]).To(Equal("GET"))
			Expect(jsonLog["rtr"].(map[string]interface{})["x_forwarded_proto"]).To(Equal("https"))
			Expect(jsonLog["rtr"].(map[string]interface{})["status"]).To(Equal(float64(200)))
		})
	})

	Context("Parse Metric Logs", func() {

		It("returns expected gauge fields", func() {
			// timestamp org/space/app app timestamp
			currentTime := time.Now()
			timestamp := currentTime.Format("2006-01-02T15:04:05.999999Z")
			rtrTpl := `<14>1 %s %s.%s.%s - [metrics] - [gauge@47450 name="memory" value="5423" unit="bytes"] - %s.hbx.geo.francetelecom.fr:443`
			rtrMsg := fmt.Sprintf(rtrTpl, timestamp, org, space, app, app)
			metadata := getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, []byte(rtrMsg), programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["app_instance"]).To(Equal(float64(0)))

			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("metrics"))
			Expect(jsonLog["@type"].(string)).To(Equal("Metrics"))
			Expect(jsonLog["@metric"].(map[string]interface{})["name"]).To(Equal("memory"))
			Expect(jsonLog["@metric"].(map[string]interface{})["type"]).To(Equal("gauge"))
			Expect(jsonLog["@metric"].(map[string]interface{})["unit"]).To(Equal("bytes"))
			Expect(jsonLog["@metric"].(map[string]interface{})["value"]).To(Equal(float64(5423)))
		})

		It("returns expected counter fields", func() {
			// timestamp org/space/app app timestamp
			currentTime := time.Now()
			timestamp := currentTime.Format("2006-01-02T15:04:05.999999Z")
			rtrTpl := `<14>1 %s %s.%s.%s - [metrics] - [counter@47450 name="connexion-error" total="99" delta="1"] - %s.hbx.geo.francetelecom.fr:443`
			rtrMsg := fmt.Sprintf(rtrTpl, timestamp, org, space, app, app)
			metadata := getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, []byte(rtrMsg), programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["app_instance"]).To(Equal(float64(0)))

			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("metrics"))
			Expect(jsonLog["@type"].(string)).To(Equal("Metrics"))
			Expect(jsonLog["@metric"].(map[string]interface{})["name"]).To(Equal("connexion-error"))
			Expect(jsonLog["@metric"].(map[string]interface{})["type"]).To(Equal("counter"))
			Expect(jsonLog["@metric"].(map[string]interface{})["total"]).To(Equal(float64(99)))
			Expect(jsonLog["@metric"].(map[string]interface{})["delta"]).To(Equal(float64(1)))
		})

		It("returns expected timer fields", func() {
			// timestamp org/space/app app timestamp
			currentTime := time.Now()
			timestamp := currentTime.Format("2006-01-02T15:04:05.999999Z")
			rtrTpl := `<14>1 %s %s.%s.%s - [metrics] - [timer@47450 name="my-timer" start="0" stop="10"] - %s.hbx.geo.francetelecom.fr:443`
			rtrMsg := fmt.Sprintf(rtrTpl, timestamp, org, space, app, app)
			metadata := getMetadata(org_id, space_id, app_id)

			parsed, err := gParser.Parse(metadata, []byte(rtrMsg), programPatterns)
			Expect(err).ToNot(HaveOccurred())
			jsonLog := make(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal([]byte(*parsed.Message), &jsonLog)
			Expect(err).ToNot(HaveOccurred())

			Expect(jsonLog["@cf"].(map[string]interface{})["app_id"]).To(Equal(app_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["app"]).To(Equal(app))
			Expect(jsonLog["@cf"].(map[string]interface{})["org_id"]).To(Equal(org_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["org"]).To(Equal(org))
			Expect(jsonLog["@cf"].(map[string]interface{})["space_id"]).To(Equal(space_id))
			Expect(jsonLog["@cf"].(map[string]interface{})["space"]).To(Equal(space))
			Expect(jsonLog["@cf"].(map[string]interface{})["app_instance"]).To(Equal(float64(0)))

			Expect(jsonLog["@source"].(map[string]interface{})["type"]).To(Equal("metrics"))
			Expect(jsonLog["@type"].(string)).To(Equal("Metrics"))
			Expect(jsonLog["@metric"].(map[string]interface{})["name"]).To(Equal("my-timer"))
			Expect(jsonLog["@metric"].(map[string]interface{})["type"]).To(Equal("timer"))
			Expect(jsonLog["@metric"].(map[string]interface{})["start"]).To(Equal(float64(0)))
			Expect(jsonLog["@metric"].(map[string]interface{})["stop"]).To(Equal(float64(10)))
		})
	})
})
