package parser

import (
	"encoding/json"
	"fmt"
	"github.com/ArthurHlt/grok"
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/tpl"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
	"strings"
)

const defCompanyId = "logsbroker@1368"

type Parser struct {
	filters []Filter
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
	data := make(map[string]string)
	if prevData, ok := p[cid]; ok {
		data = prevData
	}
	data[key] = val
	p[cid] = data
	return p
}

func NewParser(parsingKeys []model.ParsingKey) *Parser {
	grokParser, _ := grok.NewWithConfig(&grok.Config{
		NamedCapturesOnly: true,
	})
	grokParser.AddPatternsFromMap(patterns)
	grokParser.AddPatternsFromMap(programPatternsToGrokPattern())
	return &Parser{
		filters: []Filter{
			&DefaultFilter{grokParser},
			&RtrFilter{grokParser},
			&AppFilter{grokParser, append(parsingKeys, defaultParsingKeys...)},
		},
	}
}

func (p Parser) Parse(logData model.LogMetadata, message []byte, patterns ...string) (*rfc5424.SyslogMessage, error) {
	p5424 := rfc5424.NewParser()
	pMes, err := p5424.Parse(message, nil)
	if err != nil {
		return nil, err
	}
	if pMes.Message() == nil || strings.TrimSpace(*pMes.Message()) == "" {
		return nil, nil
	}

	org, space, app := p.ParseHost(pMes)

	cId := logData.InstanceParam.CompanyID
	if cId == "" {
		cId = defCompanyId
	}

	tags := model.Labels(logData.InstanceParam.Tags).ToMap()
	data := make(map[string]interface{})
	pMes.SetParameter("ensure-init-data@0", "foo", "bar")
	msgParam := MsgParam(*pMes.StructuredData())
	delete(msgParam, "ensure-init-data@0")
	msgParam.
		SetParameter(cId, "app", fmt.Sprintf("%s/%s/%s", org, space, app)).
		SetParameter(cId, "space", space).
		SetParameter(cId, "space_id", logData.InstanceParam.SpaceID).
		SetParameter(cId, "org_id", logData.InstanceParam.OrgID).
		SetParameter(cId, "app_id", logData.AppID).
		SetParameter(cId, "org", org).
		SetParameter(cId, "app_name", app)

	for _, filter := range p.filters {
		if !filter.Match(pMes) {
			continue
		}
		var values map[string]interface{}
		if _, ok := filter.(FilterPatterns); ok && len(patterns) > 0 {
			values = filter.(FilterPatterns).FilterPatterns(pMes, patterns)
		} else {
			values = filter.Filter(pMes)
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
		msgParam.SetParameter(cId, k, v)
	}
	b, _ := json.Marshal(data)
	structDataPtr := pMes.StructuredData()
	*structDataPtr = msgParam
	pMes.SetMessage(string(b) + "\n")

	return pMes, nil
}

func (p Parser) ParseHost(pmes *rfc5424.SyslogMessage) (org, space, app string) {
	s := strings.Split(*pmes.Hostname(), ".")
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
	pMes, err := p5424.Parse(message, nil)
	if err != nil {
		return "", "", ""
	}

	return p.ParseHost(pMes)
}
