package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/pivotal-cf/brokerapi/domain"
	"net/url"
)

const serviceId = "11c147f0-297f-4fd6-9401-e94e64f37094"

type LoghostBroker struct {
	db     *gorm.DB
	config model.Config
	cacher *MetaCacher
}

func NewLoghostBroker(db *gorm.DB, cacher *MetaCacher, config model.Config) *LoghostBroker {
	return &LoghostBroker{
		db:     db,
		config: config,
		cacher: cacher,
	}
}

func (b LoghostBroker) Services(ctx context.Context) ([]domain.Service, error) {

	return []domain.Service{{
		ID:          serviceId,
		Name:        "logs",
		Description: "Drain apps logs to a or multiple syslog server(s).",
		Bindable:    true,
		Requires:    []domain.RequiredPermission{domain.PermissionSyslogDrain},
		Plans:       model.SyslogAddresses(b.config.SyslogAddresses).ToServicePlans(),
		Metadata: &domain.ServiceMetadata{
			DisplayName:         "logs",
			LongDescription:     "Drain apps logs to a or multiple syslog server(s).",
			DocumentationUrl:    "",
			SupportUrl:          "",
			ImageUrl:            "",
			ProviderDisplayName: "Orange",
		},
		InstancesRetrievable: true,
		BindingsRetrievable:  true,
		Tags: []string{
			"syslog",
			"forward",
		},
	}}, nil
}

func (b LoghostBroker) Provision(_ context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {
	syslogAddr, err := b.foundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
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
		Revision:   0,
	})
	return domain.ProvisionedServiceSpec{}, nil
}

func (b LoghostBroker) foundSyslogWriter(planIDOrName string) (model.SyslogAddress, error) {
	for _, addr := range b.config.SyslogAddresses {
		if addr.ID == planIDOrName || addr.Name == planIDOrName {
			return addr, nil
		}
	}
	return model.SyslogAddress{}, fmt.Errorf("Cannot found syslog writer for plan id or name '%s'.", planIDOrName)
}

func (b LoghostBroker) Deprovision(ctx context.Context, instanceID string, details domain.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	b.db.Delete(model.LogMetadata{}, "instance_id = ?", instanceID)
	return domain.DeprovisionServiceSpec{}, nil
}

func (b LoghostBroker) Bind(_ context.Context, instanceID, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {
	var instanceParam model.InstanceParam
	var ctx model.ContextBind
	json.Unmarshal([]byte(details.RawContext), &ctx)

	var params model.BindingParams
	json.Unmarshal(details.RawParameters, &params)

	b.db.First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.Binding{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	appGuid := ctx.AppGUID
	if appGuid == "" {
		appGuid = details.AppGUID
	}

	syslogAddr, err := b.foundSyslogWriter(instanceParam.SyslogName)
	if err != nil {
		return domain.Binding{}, err
	}

	b.db.Create(&model.LogMetadata{
		BindingID:    bindingID,
		InstanceID:   instanceID,
		AppID:        appGuid,
		Patterns:     createPatterns(params.Patterns),
		Tags:         model.MapToTags(params.Tags),
		SourceLabels: model.MapToTags(syslogAddr.SourceLabels),
	})

	url, _ := url.Parse(b.config.SyslogDrainURL)
	if b.config.VirtualHost {
		return domain.Binding{
			SyslogDrainURL: fmt.Sprintf("%s://%s.%s", url.Scheme, bindingID, url.Host),
		}, nil
	}
	return domain.Binding{
		SyslogDrainURL: fmt.Sprintf("%s://%s/%s", url.Scheme, url.Host, bindingID),
	}, nil
}

func (b LoghostBroker) Unbind(ctx context.Context, instanceID, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	b.db.Delete(model.LogMetadata{}, "binding_id = ?", bindingID)
	return domain.UnbindSpec{}, nil
}

func (b LoghostBroker) Update(_ context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {
	syslogAddr, err := b.foundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.UpdateServiceSpec{}, err
	}

	var instanceParam model.InstanceParam
	b.db.Set("gorm:auto_preload", true).First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.UpdateServiceSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
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

	b.db.Delete(instanceParam.Tags)
	b.db.Delete(instanceParam.Patterns)

	b.db.Save(&model.InstanceParam{
		InstanceID: instanceID,
		SpaceID:    instanceParam.SpaceID,
		OrgID:      instanceParam.OrgID,
		Namespace:  instanceParam.Namespace,
		SyslogName: syslogAddr.Name,
		Patterns:   createPatterns(params.Patterns),
		Tags:       model.MapToTags(tags),
		CompanyID:  syslogAddr.CompanyID,
		Revision:   instanceParam.Revision + 1,
	})
	return domain.UpdateServiceSpec{}, nil
}

func (LoghostBroker) LastOperation(ctx context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}

func (b LoghostBroker) GetInstance(_ context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	var instanceParam model.InstanceParam

	b.db.Set("gorm:auto_preload", true).First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.GetInstanceDetailsSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	syslogAddr, err := b.foundSyslogWriter(instanceParam.SyslogName)
	if err != nil {
		return domain.GetInstanceDetailsSpec{}, err
	}

	patterns := make([]string, len(instanceParam.Patterns))
	for i, pattern := range instanceParam.Patterns {
		patterns[i] = pattern.Pattern
	}
	return domain.GetInstanceDetailsSpec{
		PlanID:    syslogAddr.ID,
		ServiceID: serviceId,
		Parameters: model.ProvisionParams{
			Tags:     model.Labels(instanceParam.Tags).ToMap(),
			Patterns: patterns,
		},
	}, nil
}

func (b LoghostBroker) GetBinding(_ context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	var logData model.LogMetadata
	b.db.Set("gorm:auto_preload", true).First(&logData, "binding_id = ?", bindingID)
	if logData.BindingID == "" {
		return domain.GetBindingSpec{}, fmt.Errorf("binding id '%s' not found", bindingID)
	}

	urlDrain, _ := url.Parse(b.config.SyslogDrainURL)
	syslogDrainURl := fmt.Sprintf("%s://%s/%s", urlDrain.Scheme, urlDrain.Host, bindingID)
	if b.config.VirtualHost {
		syslogDrainURl = fmt.Sprintf("%s://%s.%s", urlDrain.Scheme, bindingID, urlDrain.Host)
	}
	syslogDrainURl += fmt.Sprintf("?%s=%d", model.RevKey, logData.InstanceParam.Revision)
	patterns := make([]string, len(logData.Patterns))
	for i, pattern := range logData.Patterns {
		patterns[i] = pattern.Pattern
	}
	return domain.GetBindingSpec{
		SyslogDrainURL: syslogDrainURl,
		Parameters: model.ProvisionParams{
			Tags:     model.Labels(logData.Tags).ToMap(),
			Patterns: patterns,
		},
	}, nil
}

func (b LoghostBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
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
