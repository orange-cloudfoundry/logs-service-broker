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
	g           *grok.Grok
	parsingKeys []string
}

var regexJson = regexp.MustCompile(`^\s*{\s*".*}\s*$`)

func (f *AppFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	return f.FilterPatterns(pMes, []string{})
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
		return resultMap
	}
	if appData, ok := resultMap["app"]; ok {
		resultMap["@app"] = appData
	}
	return resultMap
}

func (f *AppFilter) FilterPatterns(pMes *rfc5424.SyslogMessage, patterns []string) map[string]interface{} {
	return f.filterPatternsMsg(*pMes.Message(), patterns)
}

func (f *AppFilter) filterPatternsMsg(message string, patterns []string) map[string]interface{} {
	if regexJson.MatchString(message) {
		return f.filterJson(message)
	}
	resultMap := make(map[string]interface{})
	for _, pattern := range patterns {
		values, _ := f.g.ParseTyped(
			pattern,
			message,
		)
		if len(values) == 0 {
			continue
		}
		resultMap = Mapper(values)
		break
	}
	if len(resultMap) == 0 {
		resultMap = f.filterProgramPattern(message)
	}
	if len(resultMap) == 0 {
		return map[string]interface{}{
			"@message": message,
		}
	}
	resultMap = f.parseJsonMapValue(resultMap)
	msgKey, textValue := f.findTextValue(resultMap, f.parsingKeys)
	if textValue != "" {
		resultMap = utils.MergeMap(resultMap, f.filterPatternsMsg(textValue, patterns))
	}
	msg, hasMsg := resultMap["@message"]
	if hasMsg {
		resultMap[msgKey] = fmt.Sprint(msg)
	} else {
		resultMap["@message"] = ""
	}
	return resultMap
}

func (f *AppFilter) findTextValue(m map[string]interface{}, possibleKeys []string) (key, value string) {
	for _, key := range possibleKeys {
		v := utils.FoundVarDelim(m, key)
		if v == nil {
			continue
		}
		if txt, ok := v.(string); ok {
			return key, txt
		}
	}
	return "", ""
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
