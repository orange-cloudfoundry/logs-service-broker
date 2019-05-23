package parser

import "fmt"

var patterns = map[string]string{
	"GREEDYQUOTE":        `[^"]*`,
	"MODSECCLIENT":       `\[client %{IPORHOST:[modsecurity-error][sourcehost]}\]`,
	"MODSECPREFIX":       `ModSecurity: %{NOTSPACE:[modsecurity-error][severity]}\. %{GREEDYDATA:[modsecurity-error][message]}`,
	"MODSECRULEFILE":     `\[file %{QUOTEDSTRING:[modsecurity-error][rulefile]}\]`,
	"MODSECRULELINE":     `\[line %{QUOTEDSTRING:[modsecurity-error][ruleline]}\]`,
	"MODSECMATCHOFFSET":  `\[offset %{QUOTEDSTRING:[modsecurity-error][matchoffset]}\]`,
	"MODSECRULEID":       `\[id \"%{NUMBER:[modsecurity-error][ruleid]:int}\"\]`,
	"MODSECRULEREV":      `\[rev %{QUOTEDSTRING:[modsecurity-error][rulerev]}\]`,
	"MODSECRULEMSG":      `\[msg %{QUOTEDSTRING:[modsecurity-error][rulemessage]}\]`,
	"MODSECRULEDATA":     `\[data %{QUOTEDSTRING:[modsecurity-error][ruledata]}\]`,
	"MODSECRULESEVERITY": `\[severity %{QUOTEDSTRING:[modsecurity-error][ruleseverity]}\]`,
	"MODSECRULEVERSION":  `\[ver %{QUOTEDSTRING:[modsecurity-error][ruleversion]}\]`,
	"MODSECRULEMATURITY": `\[maturity %{QUOTEDSTRING:[modsecurity-error][rulematurity]}\]`,
	"MODSECRULEACCURACY": `\[accuracy %{QUOTEDSTRING:[modsecurity-error][ruleaccuracy]}\]`,
	"MODSECRULETAGS":     `(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag0]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag1]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag2]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag3]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag4]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag5]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag6]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag7]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag8]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag9]}\] )?(?:\[tag %{QUOTEDSTRING}\] )*`,
	"MODSECHOSTNAME":     `\[hostname %{QUOTEDSTRING:[modsecurity-error][targethost]}\]`,
	"MODSECURI":          `\[uri %{QUOTEDSTRING:[modsecurity-error][targeturi]}\]`,
	"MODSECUID":          `\[unique_id %{QUOTEDSTRING:[modsecurity-error][uniqueid]}\]`,
	"MODSECAPACHEERROR":  `%{MODSECCLIENT} %{MODSECPREFIX} %{MODSECRULEFILE} %{MODSECRULELINE} (?:%{MODSECMATCHOFFSET} )?(?:%{MODSECRULEID} )?(?:%{MODSECRULEREV} )?(?:%{MODSECRULEMSG} )?(?:%{MODSECRULEDATA} )?(?:%{MODSECRULESEVERITY} )?(?:%{MODSECRULEVERSION} )?(?:%{MODSECRULEMATURITY} )?(?:%{MODSECRULEACCURACY} )?%{MODSECRULETAGS}%{MODSECHOSTNAME} %{MODSECURI} %{MODSECUID}`,
	"MODSECSCOREERROR":   `Inbound Anomaly Score Exceeded \(Total Inbound Score: %{NUMBER:[modsecurity-error][score]:int}%{GREEDYDATA}`,
	"MODSECSCOREAUDIT":   `Inbound Anomaly Score Exceeded \(Total Inbound Score: %{NUMBER:[modsecurity-audit][score]:int}%{GREEDYDATA}`,
	"RTR":                `%{HOSTNAME:hostname} - \[%{TIMESTAMP_ISO8601:timestamp}\] "%{WORD:verb} %{URIPATHPARAM:path} %{PROG:http_spec}" %{BASE10NUM:status:int} %{BASE10NUM:request_bytes_received:int} %{BASE10NUM:body_bytes_sent:int} "%{GREEDYQUOTE:referer}" "%{GREEDYQUOTE:http_user_agent}" "%{IPORHOST:src_host}:%{POSINT:src_port:int}" "%{IPORHOST:dst_host}:%{POSINT:dst_port:int}" x_forwarded_for:"%{GREEDYQUOTE:x_forwarded_for}" x_forwarded_proto:"%{GREEDYQUOTE:x_forwarded_proto}" vcap_request_id:"%{NOTSPACE:vcap_request_id}" response_time:%{NUMBER:response_time_sec:float} app_id:"%{NOTSPACE:app_id}" app_index:"%{BASE10NUM:app_index:int}"`,
	"DATESTAMP_ALT":      `%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{YEAR}`,
	"DATESTAMP_TXT":      `%{MONTHDAY}-%{MONTH}-%{YEAR} %{TIME}`,
}

var programPatterns = []string{
	`%{TIME} \|\-%{LOGLEVEL:@level} in %{NOTSPACE:[app][logger]} - %{GREEDYDATA:@message}`,
	`\[CONTAINER\]%{SPACE}%{NOTSPACE}%{SPACE}%{LOGLEVEL:@level}%{SPACE}%{GREEDYDATA:@message}`,
	`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}%{HOSTNAME:[app][hostname]} - - \[%{HTTPDATE:[app][timestamp]}\] "%{WORD:[app][verb]} %{URIPATHPARAM:[app][path]} %{PROG:[app][http_spec]}" %{BASE10NUM:[app][status]:int} %{BASE10NUM:[app][request_bytes_received]:int} vcap_request_id=%{NOTSPACE:@request_id} %{GREEDYDATA:@message}`,
	`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_ALT:[app][timestamp]}\] \[(core|mpm_event):%{WORD:@level}\] %{GREEDYDATA:@message}`,
	`%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_TXT:[app][timestamp]}\] %{LOGLEVEL:@level}: %{GREEDYDATA:@message}`,
}

func programPatternsToGrokPattern() map[string]string {
	d := make(map[string]string)
	for i, p := range programPatterns {
		d[fmt.Sprintf("PG%d", i)] = p
	}
	return d
}
