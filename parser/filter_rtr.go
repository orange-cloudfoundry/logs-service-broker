package parser

import (
	"fmt"
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
	"github.com/vjeantet/grok"
	"regexp"
	"strings"
)

type RtrFilter struct {
	g *grok.Grok
}

func (f *RtrFilter) SetGrok(g *grok.Grok) {
	f.g = g
}

func (f RtrFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	data := make(map[string]interface{})
	values, err := f.g.ParseTyped("%{RTR}", *pMes.Message())
	if err != nil {
		return map[string]interface{}{"@message": *pMes.Message(), "@exception": err.Error()}
	}
	dataRtr := make(map[string]interface{})
	dataRtr["hostname"] = values["hostname"]
	dataRtr["timestamp"] = values["timestamp"]
	dataRtr["verb"] = values["verb"]
	dataRtr["path"] = values["path"]
	dataRtr["http_spec"] = values["http_spec"]
	dataRtr["status"] = values["status"]
	dataRtr["request_bytes_received"] = values["request_bytes_received"]
	dataRtr["body_bytes_sent"] = values["body_bytes_sent"]
	dataRtr["referer"] = values["referer"]
	dataRtr["http_user_agent"] = values["http_user_agent"]
	dataRtr["src"] = map[string]interface{}{
		"host": values["src_host"],
		"port": values["src_port"],
	}
	dataRtr["dst"] = map[string]interface{}{
		"host": values["dst_host"],
		"port": values["dst_port"],
	}
	xff := strings.Split(values["x_forwarded_for"].(string), ", ")
	if len(xff) == 1 {
		xff = strings.Split(values["x_forwarded_for"].(string), ",")
	}
	dataRtr["x_forwarded_for"] = xff
	dataRtr["remote_addr"] = xff[0]
	dataRtr["x_forwarded_proto"] = values["x_forwarded_proto"]
	dataRtr["vcap_request_id"] = values["vcap_request_id"]
	dataRtr["response_time_sec"] = values["response_time_sec"]
	respSec := utils.RoundPlus(values["response_time_sec"].(float64), 3)
	dataRtr["response_time_ms"] = int64(respSec * 1000)
	dataRtr["app_id"] = values["app_id"]
	dataRtr["app_index"] = values["app_index"]
	data["rtr"] = dataRtr

	data["@request_id"] = values["vcap_request_id"]

	data["@message"] = fmt.Sprintf(
		"%d %s %s (%d ms)",
		values["status"],
		values["verb"],
		values["path"],
		dataRtr["response_time_ms"],
	)

	data["@level"] = "INFO"
	if values["status"].(int) == 400 {
		data["@level"] = "ERROR"
	}
	return data
}

func (RtrFilter) Match(pMes *rfc5424.SyslogMessage) bool {
	r := regexp.MustCompile(`^\[RTR/[0-9]+\]`)
	return r.MatchString(*pMes.ProcID())
}
