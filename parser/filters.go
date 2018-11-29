package parser

import (
	"github.com/influxdata/go-syslog/rfc5424"
)

type Filter interface {
	Filter(pMes *rfc5424.SyslogMessage) map[string]interface{}
	Match(pMes *rfc5424.SyslogMessage) bool
}

type FilterPatterns interface {
	FilterPatterns(pMes *rfc5424.SyslogMessage, patterns []string) map[string]interface{}
}
