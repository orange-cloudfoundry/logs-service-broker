package parser

import (
	"encoding/json"
	"fmt"
	"github.com/influxdata/go-syslog/rfc5424"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/tpl"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
	"github.com/vjeantet/grok"
	"strings"
)

const defCompanyId = "logsbroker@1368"

type Parser struct {
	p5424   *rfc5424.Parser
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
}

func NewParser() *Parser {
	grokParser, _ := grok.NewWithConfig(&grok.Config{
		NamedCapturesOnly: true,
	})
	grokParser.AddPatternsFromMap(patterns)
	grokParser.AddPatternsFromMap(programPatternsToGrokPattern())
	return &Parser{
		p5424: rfc5424.NewParser(),
		filters: []Filter{
			&DefaultFilter{grokParser},
			&RtrFilter{grokParser},
			&AppFilter{grokParser},
		},
	}
}

func (p Parser) Parse(logData model.LogMetadata, message []byte, patterns ...string) (*rfc5424.SyslogMessage, error) {
	pMes, err := p.p5424.Parse(message, nil)
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
	for k, v := range model.Labels(logData.Tags).ToMap() {
		tags[k] = v
	}

	data := make(map[string]interface{})

	if len(logData.SourceLabels) > 0 {
		data["@source"] = model.Labels(logData.SourceLabels).ToMap()
	}

	tags, err = tpl.NewTemplater(TemplateData{
		Org:       org,
		OrgID:     logData.InstanceParam.OrgID,
		Space:     space,
		SpaceID:   logData.InstanceParam.SpaceID,
		Namespace: logData.InstanceParam.Namespace,
		AppID:     logData.AppID,
		App:       app,
	}).Execute(tags)
	if err != nil {
		data["@exception_tag"] = err.Error()
	}

	pMes.
		SetParameter(cId, "app", fmt.Sprintf("%s/%s/%s", org, space, app)).
		SetParameter(cId, "space", space).
		SetParameter(cId, "space_id", logData.InstanceParam.SpaceID).
		SetParameter(cId, "org_id", logData.InstanceParam.OrgID).
		SetParameter(cId, "app_id", logData.AppID).
		SetParameter(cId, "org", org).
		SetParameter(cId, "app_name", app)

	for k, v := range tags {
		pMes.SetParameter(cId, k, v)
	}

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
	b, _ := json.Marshal(data)
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
