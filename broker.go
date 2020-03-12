package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/pivotal-cf/brokerapi/domain"
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
	docsUrl := ""
	if b.config.ExternalUrl != "" {
		docsUrl = fmt.Sprintf("%s/docs", strings.TrimSuffix(b.config.ExternalUrl, "/"))
	}
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
			DocumentationUrl:    docsUrl,
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
	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	var ctx model.ContextProvision
	json.Unmarshal([]byte(details.RawContext), &ctx)

	var params model.ProvisionParams
	err = json.Unmarshal(details.RawParameters, &params)
	if err != nil && len(details.RawParameters) > 0 {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("Error when loading params: %s", err.Error())
	}

	tags := syslogAddr.Tags
	if tags == nil {
		tags = make(map[string]string)
	}

	for k, v := range params.Tags {
		tags[k] = v
	}
	drainType := syslogAddr.DefaultDrainType
	if params.DrainType != nil && *params.DrainType != "" {
		drainType = *params.DrainType
	}
	if b.config.DisableDrainType {
		drainType = ""
	}
	patterns := append(syslogAddr.Patterns, params.Patterns...)

	// clean if something exists before
	b.db.Delete(model.Pattern{}, "instance_id = ?", instanceID)
	b.db.Delete(model.Label{}, "instance_id = ?", instanceID)
	b.db.Delete(model.SourceLabel{}, "instance_id = ?", instanceID)

	b.db.Create(&model.InstanceParam{
		InstanceID:   instanceID,
		SpaceID:      ctx.SpaceGUID,
		OrgID:        ctx.OrganizationGUID,
		Namespace:    ctx.Namespace,
		SyslogName:   syslogAddr.Name,
		Patterns:     model.ListToPatterns(patterns),
		SourceLabels: model.MapToSourceLabels(syslogAddr.SourceLabels),
		Tags:         model.MapToLabels(tags),
		CompanyID:    syslogAddr.CompanyID,
		UseTls:       params.UseTLS || b.config.PreferTLS,
		DrainType:    model.DrainType(strings.ToLower(string(drainType))),
		Revision:     1,
	})
	return domain.ProvisionedServiceSpec{
		DashboardURL: b.genDashboardUrl(instanceID),
	}, nil
}

func (b LoghostBroker) Deprovision(ctx context.Context, instanceID string, details domain.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	b.db.Delete(&model.LogMetadata{
		InstanceID: instanceID,
	}, "instance_id = ?", instanceID)
	b.db.Delete(&model.InstanceParam{
		InstanceID: instanceID,
	}, "instance_id = ?", instanceID)
	b.db.Delete(&model.InstanceParam{}, "instance_id = ?", instanceID)
	return domain.DeprovisionServiceSpec{}, nil
}

func (b LoghostBroker) Bind(_ context.Context, instanceID, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {
	var instanceParam model.InstanceParam
	var ctx model.ContextBind
	json.Unmarshal([]byte(details.RawContext), &ctx)

	b.db.Order("revision desc").First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.Binding{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	appGuid := ctx.AppGUID
	if appGuid == "" {
		appGuid = details.AppGUID
	}

	b.db.Create(&model.LogMetadata{
		BindingID:  bindingID,
		InstanceID: instanceID,
		AppID:      appGuid,
	})

	syslogDrainURl := b.genUrl(instanceParam, bindingID)
	return domain.Binding{
		SyslogDrainURL: syslogDrainURl,
	}, nil
}

func (b LoghostBroker) genDashboardUrl(instanceId string) string {
	if b.config.ExternalUrl == "" {
		return ""
	}
	return fmt.Sprintf("%s/docs/%s", strings.TrimSuffix(b.config.ExternalUrl, "/"), instanceId)
}

func (b LoghostBroker) genUrl(instanceParam model.InstanceParam, bindingID string) string {
	urlSyslog, _ := url.Parse(b.config.SyslogDrainURL)
	domainURL := urlSyslog.Host
	scheme := "http"
	port := b.config.Port
	if instanceParam.UseTls && b.config.HasTLS() {
		scheme = "https"
		port = b.config.TLSPort
	}
	if urlSyslog.Host == "" {
		domainURL = b.config.SyslogDrainURL
	}
	domainURL = strings.Split(domainURL, ":")[0]

	syslogDrainURl := fmt.Sprintf("%s://%s:%d/%s", scheme, domainURL, port, bindingID)
	if b.config.VirtualHost {
		syslogDrainURl = fmt.Sprintf("%s://%s.%s:%d/", scheme, bindingID, domainURL, port)
	}
	queryValues := make(url.Values)
	queryValues.Add(model.RevKey, fmt.Sprint(instanceParam.Revision))
	if instanceParam.DrainType != "" {
		queryValues.Add(model.DrainTypeKey, string(instanceParam.DrainType))
	}
	syslogDrainURl += fmt.Sprintf("?%s", queryValues.Encode())
	return syslogDrainURl
}

func (b LoghostBroker) Unbind(ctx context.Context, instanceID, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	b.db.Delete(model.LogMetadata{}, "binding_id = ?", bindingID)
	return domain.UnbindSpec{}, nil
}

func (b LoghostBroker) Update(_ context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {
	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.UpdateServiceSpec{}, err
	}

	var instanceParam model.InstanceParam
	b.db.Set("gorm:auto_preload", true).Order("revision desc").First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.UpdateServiceSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	var ctx model.ContextProvision
	json.Unmarshal([]byte(details.RawContext), &ctx)

	var params model.ProvisionParams
	err = json.Unmarshal(details.RawParameters, &params)
	if err != nil && len(details.RawParameters) > 0 {
		return domain.UpdateServiceSpec{}, fmt.Errorf("Error when loading params: %s", err.Error())
	}
	tags := syslogAddr.Tags
	if tags == nil {
		tags = make(map[string]string)
	}

	for k, v := range params.Tags {
		tags[k] = v
	}

	b.db.Delete(model.Label{}, "instance_id = ?", instanceID)
	b.db.Delete(model.Pattern{}, "instance_id = ?", instanceID)
	b.db.Delete(model.SourceLabel{}, "instance_id = ?", instanceID)

	drainType := syslogAddr.DefaultDrainType
	if params.DrainType != nil && *params.DrainType != "" {
		drainType = *params.DrainType
	}
	if b.config.DisableDrainType {
		drainType = ""
	}
	patterns := append(syslogAddr.Patterns, params.Patterns...)
	b.db.Create(&model.InstanceParam{
		InstanceID: instanceID,
		SpaceID:    instanceParam.SpaceID,
		OrgID:      instanceParam.OrgID,
		Namespace:  instanceParam.Namespace,
		SyslogName: syslogAddr.Name,
		Patterns:   model.ListToPatterns(patterns),
		Tags:       model.MapToLabels(tags),
		CompanyID:  syslogAddr.CompanyID,
		UseTls:     params.UseTLS || b.config.PreferTLS,
		DrainType:  model.DrainType(strings.ToLower(string(drainType))),
		Revision:   instanceParam.Revision + 1,
	})
	return domain.UpdateServiceSpec{
		DashboardURL: b.genDashboardUrl(instanceID),
	}, nil
}

func (LoghostBroker) LastOperation(ctx context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}

func (b LoghostBroker) GetInstance(_ context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	var instanceParam model.InstanceParam

	b.db.Set("gorm:auto_preload", true).Order("revision desc").First(&instanceParam, "instance_id = ?", instanceID)
	if instanceParam.InstanceID == "" {
		return domain.GetInstanceDetailsSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
	}

	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(instanceParam.SyslogName)
	if err != nil {
		return domain.GetInstanceDetailsSpec{}, err
	}

	return domain.GetInstanceDetailsSpec{
		PlanID:       syslogAddr.ID,
		ServiceID:    serviceId,
		DashboardURL: b.genDashboardUrl(instanceID),
		Parameters: model.ProvisionParams{
			Tags:     model.Labels(instanceParam.Tags).ToMap(),
			Patterns: model.Patterns(instanceParam.Patterns).ToList(),
		},
	}, nil
}

func (b LoghostBroker) GetBinding(_ context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	var logData model.LogMetadata

	b.db.First(&logData, "binding_id = ?", bindingID)
	if logData.BindingID == "" {
		return domain.GetBindingSpec{}, fmt.Errorf("binding id '%s' not found", bindingID)
	}
	var instanceParam model.InstanceParam
	b.db.Set("gorm:auto_preload", true).Order("revision desc").First(&instanceParam, "instance_id = ?", logData.InstanceID)
	logData.InstanceParam = instanceParam

	syslogDrainURl := b.genUrl(instanceParam, bindingID)
	return domain.GetBindingSpec{
		SyslogDrainURL: syslogDrainURl,
	}, nil
}

func (b LoghostBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}
