package parser

import (
	"strconv"
	"strings"

	"github.com/ArthurHlt/grok"
	"github.com/influxdata/go-syslog/rfc5424"
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
	details := ""
	isCfTask := false
	if len(procSplit) > 2 {
		details = strings.Join(procSplit[1:len(procSplit)-1], "/")
		if strings.ToUpper(procSplit[1]) == "TASK" {
			isCfTask = true
		}
	}
	data["@source"] = map[string]interface{}{
		"type":    srcType,
		"details": details,
	}
	data["@shipper"] = map[string]interface{}{"name": "log-service", "priority": *pMes.Priority()}
	data["@input"] = "syslog"
	data["@type"] = "LogMessage"
	data["@timestamp"] = *pMes.Timestamp()
	if pMes.Message() != nil && strings.TrimSpace(*pMes.Message()) == "" {
		data["@level"] = "INFO"
		data["@message"] = *pMes.Message()
	}

	pData := *pMes.StructuredData()
	mesData := make(map[string]string)
	for _, vMap := range pData {
		for k, v := range vMap {
			mesData[k] = v
		}
	}
	cfData := map[string]interface{}{
		"app":          mesData["app_name"],
		"app_id":       mesData["app_id"],
		"app_instance": index,
		"org":          mesData["org"],
		"org_id":       mesData["org_id"],
		"space":        mesData["space"],
		"space_id":     mesData["space_id"],
	}
	if isCfTask {
		delete(cfData, "app_instance")
		cfData["task_id"] = index
		cfData["task_name"] = procSplit[2]
	}
	data["@cf"] = cfData

	return data
}

func (f DefaultFilter) Match(_ *rfc5424.SyslogMessage) bool {
	return true
}
