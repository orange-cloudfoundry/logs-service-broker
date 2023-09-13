package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/influxdata/go-syslog/v3/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
)

func init() {
	hostnameRegex := `(?P<hostname>\b(?:[0-9A-Za-z][0-9A-Za-z-]{0,62})(?:\.(?:[0-9A-Za-z][0-9A-Za-z-]{0,62}))*(\.?|\b))`
	portRegex := `(?P<port>:[0-9]+)?`
	timeRegex := `\[(?P<timestamp>[^\]]*)\]`
	verbRegex := `(?P<verb>[^\s]*)`
	pathRegex := `(?P<path>[^\s]*)`
	httpSpecRegex := `(?P<http_spec>[^\s]*)`
	statusRegex := `(?P<status>[0-9]+)`
	reqRecRegex := `(?P<request_bytes_received>[0-9]+)`
	bodySentRegex := `(?P<body_bytes_sent>[0-9]+)`
	refererRegex := `(?P<referer>[^\s]*)`
	userAgentRegex := `(?P<http_user_agent>[^"]*)`
	srcHostRegex := `(?P<src_host>[0-9\.]*)`
	srcPortRegex := `(?P<src_port>[0-9]+)`
	dstHostRegex := `(?P<dst_host>[0-9\.]*)`
	dstPortRegex := `(?P<dst_port>[0-9]+)`
	inlineParamsRegex := `(?P<params>.*)`
	regexRtr = regexp.MustCompile(fmt.Sprintf(`^%s%s - %s "%s %s %s" %s %s %s "%s" "%s" "%s:%s" "%s:%s" %s`,
		hostnameRegex, portRegex, timeRegex, verbRegex, pathRegex, httpSpecRegex, statusRegex,
		reqRecRegex, bodySentRegex,
		refererRegex, userAgentRegex,
		srcHostRegex, srcPortRegex, dstHostRegex, dstPortRegex,
		inlineParamsRegex,
	))
}

var regexRtr *regexp.Regexp

type RtrFilter struct {
}

func (f RtrFilter) parse(message string) (map[string]interface{}, error) {
	if !regexRtr.MatchString(message) {
		return map[string]interface{}{}, fmt.Errorf("Log router could not be parsed, probably format has changed.")
	}
	match := regexRtr.FindStringSubmatch(message)
	result := make(map[string]interface{})
	for i, name := range regexRtr.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	parsedParams := parseInlineParams(result["params"].(string))
	delete(result, "params")
	for k, v := range parsedParams {
		result[k] = v
	}
	return result, nil
}

func (f RtrFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	data := make(map[string]interface{})
	values, err := f.parse(*pMes.Message)
	if err != nil {
		return map[string]interface{}{"@message": *pMes.Message, "@exception": err.Error()}
	}
	dataRtr := make(map[string]interface{})
	dataRtr["hostname"] = values["hostname"]
	delete(values, "hostname")
	dataRtr["timestamp"] = values["timestamp"]
	delete(values, "timestamp")
	dataRtr["verb"] = values["verb"]
	delete(values, "verb")
	dataRtr["path"] = values["path"]
	delete(values, "path")
	dataRtr["http_spec"] = values["http_spec"]
	delete(values, "http_spec")

	status, _ := strconv.ParseInt(values["status"].(string), 10, 64)
	dataRtr["status"] = status
	delete(values, "status")

	bytesReceived, _ := strconv.ParseInt(values["request_bytes_received"].(string), 10, 64)
	dataRtr["request_bytes_received"] = bytesReceived
	delete(values, "request_bytes_received")

	bodyReceived, _ := strconv.ParseInt(values["body_bytes_sent"].(string), 10, 64)
	dataRtr["body_bytes_sent"] = bodyReceived
	delete(values, "body_bytes_sent")

	dataRtr["referer"] = values["referer"]
	delete(values, "referer")
	dataRtr["http_user_agent"] = values["http_user_agent"]
	delete(values, "http_user_agent")

	srcPort, _ := strconv.ParseInt(values["src_port"].(string), 10, 64)
	dataRtr["src"] = map[string]interface{}{
		"host": values["src_host"],
		"port": srcPort,
	}
	delete(values, "src_host")
	delete(values, "src_port")

	dstPort, _ := strconv.ParseInt(values["dst_port"].(string), 10, 64)
	dataRtr["dst"] = map[string]interface{}{
		"host": values["dst_host"],
		"port": dstPort,
	}
	delete(values, "dst_host")
	delete(values, "dst_port")

	xff := strings.Split(values["x_forwarded_for"].(string), ", ")
	if len(xff) == 1 {
		xff = strings.Split(values["x_forwarded_for"].(string), ",")
	}
	dataRtr["x_forwarded_for"] = xff
	dataRtr["remote_addr"] = xff[0]
	delete(values, "x_forwarded_for")

	dataRtr["x_forwarded_proto"] = values["x_forwarded_proto"]
	delete(values, "x_forwarded_proto")
	dataRtr["vcap_request_id"] = values["vcap_request_id"]
	delete(values, "vcap_request_id")

	dataRtr["response_time_sec"] = values["response_time"]
	respSec := utils.RoundPlus(values["response_time"].(float64), 3)
	dataRtr["response_time_ms"] = int64(respSec * 1000)
	delete(values, "response_time")

	if _, ok := values["gorouter_time"]; ok {
		dataRtr["gorouter_time_sec"] = values["gorouter_time"]
		routerSec := utils.RoundPlus(values["gorouter_time"].(float64), 3)
		dataRtr["gorouter_time_ms"] = int64(routerSec * 1000)
		delete(values, "gorouter_time")
	}

	if _, ok := values["app_time"]; ok {
		dataRtr["app_time_sec"] = values["app_time"]
		appSec := utils.RoundPlus(values["app_time"].(float64), 3)
		dataRtr["app_time_ms"] = int64(appSec * 1000)
		delete(values, "app_time")
	}

	dataRtr["app_id"] = values["app_id"]
	delete(values, "app_id")

	appIndex, _ := strconv.ParseInt(values["app_index"].(string), 10, 64)
	dataRtr["app_index"] = appIndex
	delete(values, "app_index")

	data["rtr"] = dataRtr

	data["@request_id"] = values["vcap_request_id"]
	delete(values, "vcap_request_id")

	for k, v := range values {
		data[k] = v
	}

	data[MessageKey] = fmt.Sprintf(
		"%d %s %s (%d ms)",
		dataRtr["status"],
		dataRtr["verb"],
		dataRtr["path"],
		dataRtr["response_time_ms"],
	)

	data["@level"] = "INFO"
	if status >= 400 {
		data["@level"] = "ERROR"
	}
	return data
}

func (RtrFilter) Match(pMes *rfc5424.SyslogMessage) bool {
	r := regexp.MustCompile(`^\[RTR/[0-9]+]`)
	return r.MatchString(*pMes.ProcID)
}
