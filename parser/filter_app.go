package parser

import (
	"encoding/json"
	"fmt"
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/vjeantet/grok"
	"regexp"
)

type AppFilter struct {
	g *grok.Grok
}

func (f *AppFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	if regexp.MustCompile(`^\s*{".*}\s*$`).MatchString(*pMes.Message()) {
		return f.filterJson(pMes)
	}
	for i, _ := range programPatterns {
		values, _ := f.g.ParseTyped(
			"%{"+fmt.Sprintf("PG%d", i)+"}",
			*pMes.Message(),
		)
		if len(values) == 0 {
			continue
		}
		return Mapper(values)
	}
	return map[string]interface{}{
		"@message": *pMes.Message(),
	}
}

func (f *AppFilter) FilterPatterns(pMes *rfc5424.SyslogMessage, patterns []string) map[string]interface{} {
	if regexp.MustCompile(`^\s*{".*}\s*$`).MatchString(*pMes.Message()) {
		return f.filterJson(pMes)
	}
	for _, pattern := range patterns {
		values, _ := f.g.ParseTyped(
			pattern,
			*pMes.Message(),
		)
		if len(values) == 0 {
			continue
		}
		return Mapper(values)
	}
	return map[string]interface{}{
		"@message": *pMes.Message(),
	}
}

func (f *AppFilter) filterJson(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	data := make(map[string]interface{})
	err := json.Unmarshal([]byte(*pMes.Message()), &data)
	if err != nil {
		data["@message"] = *pMes.Message()
		data["@exception"] = err.Error()
		return data
	}
	return map[string]interface{}{"app": data}
}

func (f *AppFilter) Match(pMes *rfc5424.SyslogMessage) bool {
	r := regexp.MustCompile(`^\[APP/[A-Z]+/[A-Z]+/[0-9]+\]`)
	return r.MatchString(*pMes.ProcID())
}
