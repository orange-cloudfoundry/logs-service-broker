package parser

import (
	"encoding/json"
	"fmt"
	"github.com/ArthurHlt/grok"
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
	"regexp"
)

type AppFilter struct {
	g *grok.Grok
}

var regexJson = regexp.MustCompile(`^\s*{\s*".*}\s*$`)

func (f *AppFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	if regexJson.MatchString(*pMes.Message()) {
		return f.filterJson(*pMes.Message())
	}
	return f.filterProgramPattern(*pMes.Message())
}

func (f *AppFilter) parseJsonMapValue(m map[string]interface{}) map[string]interface{} {
	if msgJson, ok := m["@json"]; ok {
		m = utils.MergeMap(m, f.filterJson(fmt.Sprint(msgJson)))
		delete(m, "@json")
	}
	if msg, ok := m["@message"]; ok && regexJson.MatchString(fmt.Sprint(msg)) {
		m = utils.MergeMap(m, f.filterJson(fmt.Sprint(msg)))
		delete(m, "@message")
	}
	return m
}

func (f *AppFilter) filterProgramPattern(message string) map[string]interface{} {
	resultMap := make(map[string]interface{})
	for i := range programPatterns {
		values, _ := f.g.ParseTyped(
			"%{"+fmt.Sprintf("PG%d", i)+"}",
			message,
		)
		if len(values) == 0 {
			continue
		}
		resultMap = Mapper(values)
		break
	}
	if len(resultMap) == 0 {
		return map[string]interface{}{
			"@message": message,
		}
	}
	if appData, ok := resultMap["app"]; ok {
		resultMap["@app"] = appData
	}
	return f.parseJsonMapValue(resultMap)
}

func (f *AppFilter) FilterPatterns(pMes *rfc5424.SyslogMessage, patterns []string) map[string]interface{} {
	if regexJson.MatchString(*pMes.Message()) {
		return f.filterJson(*pMes.Message())
	}
	resultMap := make(map[string]interface{})
	for _, pattern := range patterns {
		values, _ := f.g.ParseTyped(
			pattern,
			*pMes.Message(),
		)
		if len(values) == 0 {
			continue
		}
		resultMap = Mapper(values)
		break
	}
	if len(resultMap) == 0 {
		return f.filterProgramPattern(*pMes.Message())
	}

	return f.parseJsonMapValue(resultMap)
}

func (f *AppFilter) filterJson(message string) map[string]interface{} {
	data := make(map[string]interface{})
	err := json.Unmarshal([]byte(message), &data)
	if err != nil {
		data["@message"] = message
		data["@exception"] = err.Error()
		return data
	}
	return map[string]interface{}{"app": data}
}

func (f *AppFilter) Match(pMes *rfc5424.SyslogMessage) bool {
	r := regexp.MustCompile(`^\[APP/[A-Z]+/[A-Z]+/[0-9]+\]`)
	return r.MatchString(*pMes.ProcID())
}
