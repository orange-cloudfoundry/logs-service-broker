package parser_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/orange-cloudfoundry/logs-service-broker/model"

	"github.com/cloudfoundry/go-loggregator/rpc/loggregator_v2"
)

func TestParser(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parser Suite")
}

func buildLogEnvelope(
	time int64,
	id string,
	instance string,
	payload string,
	logType loggregator_v2.Log_Type,
	tags map[string]string,
) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Timestamp:  time,
		SourceId:   id,
		InstanceId: instance,
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(payload),
				Type:    logType,
			},
		},
		Tags: tags,
	}
}

func getPatterns() []model.Pattern {
	return []model.Pattern{
		{
			Pattern: "%{TIME} %{WORD:[@app][program]}%{SPACE}\\|%{SPACE}%{NOTSPACE:[@app][tags]}.json%{SPACE}%{GREEDYDATA:@message}",
		},
		{
			Pattern: "%{TIME} %{WORD:[@app][program]}%{SPACE}\\|%{SPACE}%{NOTSPACE:[@app][tags]}.txt%{SPACE}%{GREEDYDATA:[text]}",
		},
		{
			Pattern: "%{NOTSPACE:[@app][tags]}.json%{SPACE}%{GREEDYDATA:@message}",
		},
		{
			Pattern: "%{NOTSPACE:[@app][tags]}.txt%{SPACE}%{GREEDYDATA:[text]}",
		},
		{
			Pattern: "%{MODSECAPACHEERROR}%{GREEDYDATA:@message}",
		},
		{
			Pattern: "%{MODSECRULEMSG}",
		},
	}
}

func getMetadata(org_id, space_id, app_id string) *model.LogMetadata {
	return &model.LogMetadata{
		BindingID: "7891-a44c54d4",
		InstanceParam: model.InstanceParam{
			InstanceID: app_id,
			SpaceID:    space_id,
			OrgID:      org_id,
			Revision:   1,
			SyslogName: "loghost",
			CompanyID:  "tags@1368",
			UseTls:     true,
			Patterns:   getPatterns(),
			SourceLabels: []model.SourceLabel{
				{
					ID:         1396,
					Key:        "deployment",
					Value:      "sph-prod-scaleio",
					InstanceID: app_id,
				},
			},
		},
		InstanceID: "3452-124A234B",
		AppID:      app_id,
	}
}
