package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/pivotal-cf/brokerapi"

	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

type LoghostBroker struct {
	db     *gorm.DB
	config model.Config
}

func NewLoghostBroker(db *gorm.DB, config model.Config) *LoghostBroker {
	return &LoghostBroker{
		db:     db,
		config: config,
	}
}

func (b LoghostBroker) Services(ctx context.Context) ([]brokerapi.Service, error) {

	return []brokerapi.Service{{
		ID:          "11c147f0-297f-4fd6-9401-e94e64f37094",
		Name:        "logs",
		Description: "Drain apps logs to a or multiple syslog server(s).",
		Bindable:    true,
		Requires:    []brokerapi.RequiredPermission{brokerapi.PermissionSyslogDrain},
		Plans:       model.SyslogAddresses(b.config.SyslogAddresses).ToServicePlans(),
		Metadata: &brokerapi.ServiceMetadata{
			DisplayName:         "logs",
			LongDescription:     "Drain apps logs to a or multiple syslog server(s).",
			DocumentationUrl:    "",
			SupportUrl:          "",
			ImageUrl:            "",
			ProviderDisplayName: "Orange",
		},
		Tags: []string{
			"syslog",
			"forward",
		},
	}}, nil
}

func (b LoghostBroker) Provision(_ context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	syslogAddr, err := b.foundSyslogWriter(details.PlanID)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	var ctx model.ContextProvision
	json.Unmarshal([]byte(details.RawContext), &ctx)

	var params model.ProvisionParams
	json.Unmarshal(details.RawParameters, &params)

	tags := syslogAddr.Tags
	if tags == nil {
		tags = make(map[string]string)
	}

	for k, v := range params.Tags {
		tags[k] = v
	}

	b.db.Create(&model.InstanceParam{
		InstanceID: instanceID,
		SpaceID:    ctx.SpaceGUID,
		OrgID:      ctx.OrganizationGUID,
		Namespace:  ctx.Namespace,
		SyslogName: syslogAddr.Name,
		Patterns:   createPatterns(params.Patterns),
		Tags:       model.MapToTags(tags),
		CompanyID:  syslogAddr.CompanyID,
	})

	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (b LoghostBroker) foundSyslogWriter(planID string) (model.SyslogAddress, error) {
	for _, addr := range b.config.SyslogAddresses {
		if addr.ID == planID {
			return addr, nil
		}
	}
	return model.SyslogAddress{}, fmt.Errorf("Cannot found syslog writer for plan id '%s'.", planID)
}

func (b LoghostBroker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.db.Delete(model.LogMetadata{}, "instance_id = ?", instanceID)
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (b LoghostBroker) Bind(_ context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	var instanceParam model.InstanceParam
	var ctx model.ContextBind
	json.Unmarshal([]byte(details.RawContext), &ctx)

	var params model.BindingParams
	json.Unmarshal(details.RawParameters, &params)

	b.db.First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return brokerapi.Binding{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	appGuid := ctx.AppGUID
	if appGuid == "" {
		appGuid = details.AppGUID
	}

	b.db.Create(&model.LogMetadata{
		BindingID:  bindingID,
		InstanceID: instanceID,
		AppID:      appGuid,
		Patterns:   createPatterns(params.Patterns),
		Tags:       model.MapToTags(params.Tags),
	})
	if b.config.VirtualHost {
		return brokerapi.Binding{
			SyslogDrainURL: fmt.Sprintf("https://%s.%s", bindingID, b.config.Domain),
		}, nil
	}
	return brokerapi.Binding{
		SyslogDrainURL: fmt.Sprintf("https://%s/%s", b.config.Domain, bindingID),
	}, nil
}

func (b LoghostBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	b.db.Delete(model.LogMetadata{}, "binding_id = ?", bindingID)
	return nil
}

func (LoghostBroker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, nil
}

func (LoghostBroker) LastOperation(ctx context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func createPatterns(patternsStr []string) []model.Pattern {
	patterns := make([]model.Pattern, len(patternsStr))
	for i, p := range patternsStr {
		patterns[i] = model.Pattern{
			Pattern: p,
		}
	}
	return patterns
}
