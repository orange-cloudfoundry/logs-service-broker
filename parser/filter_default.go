package parser

import (
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/vjeantet/grok"
	"strconv"
	"strings"
)

type DefaultFilter struct {
	g *grok.Grok
}

func (f DefaultFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	data := make(map[string]interface{})

	srcType := strings.Replace(*pMes.ProcID(), "[", "", 1)
	srcType = strings.Replace(srcType, "]", "", 1)
	procSplit := strings.Split(srcType, "/")

	indexStr := procSplit[len(procSplit)-1]
	if indexStr == "" {
		indexStr = "0"
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		index = 0
	}
	srcType = procSplit[0]

	data["@source"] = map[string]interface{}{"type": strings.ToLower(srcType)}

	data["@shipper"] = map[string]interface{}{"name": "log-service", "priority": *pMes.Priority()}
	data["@input"] = "syslog"
	data["@type"] = "LogMessage"
	data["@level"] = "INFO"
	data["@timestamp"] = *pMes.Timestamp()
	data["@message"] = *pMes.Message()

	pData := *pMes.StructuredData()
	var cId string
	for k, _ := range pData {
		cId = k
		break
	}
	mesData := pData[cId]

	data["@cf"] = map[string]interface{}{
		"app":          mesData["app_name"],
		"app_id":       mesData["app_id"],
		"app_instance": index,
		"org":          mesData["org"],
		"org_id":       mesData["org_id"],
		"space":        mesData["space"],
		"space_id":     mesData["space_id"],
	}

	return data
}

func (f DefaultFilter) Match(_ *rfc5424.SyslogMessage) bool {
	return true
}
