package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ArthurHlt/grok"
	"github.com/influxdata/go-syslog/v3"
	"github.com/influxdata/go-syslog/v3/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/tpl"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
)

const defCompanyID = "logsbroker@1368"

// this is from https://github.com/cloudfoundry/cf-syslog-drain-release/blob/f13fd13ec6d08822f261cbb575aa5f357ab32f0d/src/adapter/internal/egress/tcp.go#L21-L24
// gaugeStructuredDataID contains the registered enterprise ID for the Cloud
// Foundry Foundation.
// See: https://www.iana.org/assignments/enterprise-numbers/enterprise-numbers
const (
	gaugeStructuredDataID   = "gauge@47450"
	counterStructuredDataID = "counter@47450"
	timerStructuredDataID   = "timer@47450"
	tagsStructuredDataID    = "tags@47450"
)

type Parser struct {
	filters                  []Filter
	p5424                    syslog.Machine
	ignoreTagsStructuredData bool
}

type TemplateData struct {
	Org       string
	OrgID     string
	Space     string
	SpaceID   string
	App       string
	AppID     string
	Namespace string
	Logdata   map[string]interface{}
}

type MsgParam map[string]map[string]string

var defaultParsingKeys = []model.ParsingKey{
	{
		Name: "@message",
	},
	{
		Name: "@raw",
	},
	{
		Name: "text",
	},
}

func (p MsgParam) SetParameter(cid, key, val string) MsgParam {
	var (
		data map[string]string
		ok   bool
	)
	if data, ok = p[cid]; !ok {
		data = make(map[string]string)
		p[cid] = data
	}
	data[key] = val
	return p
}

func NewParser(parsingKeys []model.ParsingKey, ignoreTagsStructuredData bool) *Parser {
	grokParser, _ := grok.NewWithConfig(&grok.Config{
		NamedCapturesOnly: true,
	})
	err := grokParser.AddPatternsFromMap(patterns)
	if err != nil {
		panic(err)
	}
	err = grokParser.AddPatternsFromMap(programPatternsToGrokPattern())
	if err != nil {
		panic(err)
	}
	return &Parser{
		ignoreTagsStructuredData: ignoreTagsStructuredData,
		p5424:                    rfc5424.NewParser(),
		filters: []Filter{
			&DefaultFilter{grokParser},
			&MetricsFilter{},
			&RtrFilter{},
			&AppFilter{grokParser, append(parsingKeys, defaultParsingKeys...)},
		},
	}
}

func (p Parser) Parse(
	logData *model.LogMetadata,
	message []byte,
	patterns []string,
) (*rfc5424.SyslogMessage, error) {
	parsedRaw, err := p.p5424.Parse(message)
	if err != nil {
		return nil, err
	}
	parsed := parsedRaw.(*rfc5424.SyslogMessage)
	if parsed.Message == nil || strings.TrimSpace(*parsed.Message) == "" {
		if !isMetrics(parsed) {
			return nil, nil
		}
	}

	org, space, app := p.ParseHost(parsed)
	compID := logData.InstanceParam.CompanyID
	if compID == "" {
		compID = defCompanyID
	}

	tags := logData.InstanceParam.TagsToMap()
	data := make(map[string]interface{})
	parsed.SetParameter("ensure-init-data@0", "foo", "bar")
	msgParam := MsgParam(*parsed.StructuredData)
	delete(msgParam, "ensure-init-data@0")
	if p.ignoreTagsStructuredData {
		delete(msgParam, tagsStructuredDataID)
	}
	msgParam.
		SetParameter(compID, "app", fmt.Sprintf("%s/%s/%s", org, space, app)).
		SetParameter(compID, "space", space).
		SetParameter(compID, "space_id", logData.InstanceParam.SpaceID).
		SetParameter(compID, "org_id", logData.InstanceParam.OrgID).
		SetParameter(compID, "app_id", logData.AppID).
		SetParameter(compID, "org", org).
		SetParameter(compID, "app_name", app)

	for _, filter := range p.filters {
		if !filter.Match(parsed) {
			continue
		}
		var values map[string]interface{}
		if _, ok := filter.(FilterPatterns); ok && len(patterns) > 0 {
			values = filter.(FilterPatterns).FilterPatterns(parsed, patterns)
		} else {
			values = filter.Filter(parsed)
		}
		data = utils.MergeMap(data, values)
	}
	if len(logData.InstanceParam.SourceLabels) > 0 {
		currentSource := make(map[string]interface{})
		if _, ok := data["@source"]; ok {
			if dataMap, ok := data["@source"].(map[string]interface{}); ok {
				currentSource = dataMap
			}
		}
		sourceLabelMap := make(map[string]interface{})
		for k, v := range model.SourceLabels(logData.InstanceParam.SourceLabels).ToMap() {
			sourceLabelMap[k] = v
		}
		data["@source"] = utils.MergeMap(sourceLabelMap, currentSource)
	}
	tags, err = tpl.NewTemplater(TemplateData{
		Org:       org,
		OrgID:     logData.InstanceParam.OrgID,
		Space:     space,
		SpaceID:   logData.InstanceParam.SpaceID,
		Namespace: logData.InstanceParam.Namespace,
		AppID:     logData.AppID,
		App:       app,
		Logdata:   data,
	}).Execute(tags)
	if err != nil {
		data["@exception_tag"] = err.Error()
	}
	for k, v := range tags {
		msgParam.SetParameter(compID, k, v)
	}
	b, _ := json.Marshal(data)
	structDataPtr := parsed.StructuredData
	*structDataPtr = msgParam
	parsed.SetMessage(string(b) + "\n")
	return parsed, nil
}

func (p Parser) ParseHost(parsed *rfc5424.SyslogMessage) (org, space, app string) {
	s := strings.Split(*parsed.Hostname, ".")
	if len(s) == 1 {
		return "", "", s[0]
	}
	if len(s) == 2 {
		return s[0], s[1], ""
	}
	return s[0], s[1], strings.Join(s[2:], ".")
}

func (p Parser) ParseHostFromMessage(message []byte) (org, space, app string) {
	p5424 := rfc5424.NewParser()
	parsedRaw, err := p5424.Parse(message)
	if err != nil {
		return "", "", ""
	}

	return p.ParseHost(parsedRaw.(*rfc5424.SyslogMessage))
}

func isMetrics(parsed *rfc5424.SyslogMessage) bool {
	structData := parsed.StructuredData
	if structData == nil || *structData == nil {
		return false
	}
	if _, ok := (*structData)[gaugeStructuredDataID]; ok {
		return true
	}
	if _, ok := (*structData)[counterStructuredDataID]; ok {
		return true
	}
	if _, ok := (*structData)[timerStructuredDataID]; ok {
		return true
	}
	return false
}

func parseInlineParams(inlineParams string) map[string]interface{} {
	inlineParams = strings.TrimSpace(inlineParams)
	splitParams := strings.Split(inlineParams, " ")
	finalParams := make(map[string]interface{})
	for _, p := range splitParams {
		slitP := strings.SplitN(p, ":", 2)
		if len(slitP) < 2 {
			continue
		}
		k := slitP[0]
		vStr := strings.TrimSpace(slitP[1])
		if vStr[0] == '"' {
			finalParams[k] = vStr[1 : len(vStr)-1]
			continue
		}
		flo, err := strconv.ParseFloat(vStr, 64)
		if err == nil {
			finalParams[k] = flo
			continue
		}
		in, err := strconv.ParseInt(vStr, 10, 64)
		if err == nil {
			finalParams[k] = in
			continue
		}
		finalParams[k] = vStr
	}
	return finalParams
}
