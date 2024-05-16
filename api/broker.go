package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/orange-cloudfoundry/logs-service-broker/dbservices"
	"github.com/orange-cloudfoundry/logs-service-broker/utils"

	"github.com/jinzhu/gorm"
	"github.com/pivotal-cf/brokerapi/domain"
	log "github.com/sirupsen/logrus"

	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

const serviceId = "11c147f0-297f-4fd6-9401-e94e64f37094"

type LoghostBroker struct {
	db     *gorm.DB
	config *model.Config
	cacher dbservices.Cacher
}

func NewLoghostBroker(
	db *gorm.DB,
	cacher dbservices.Cacher,
	config *model.Config,
) *LoghostBroker {
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
			DocumentationUrl:    b.genDocURL(),
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

func (b LoghostBroker) newDBError(phase string, err error) error {
	nerr := fmt.Errorf("unexpected database error while %s: %s", phase, err.Error())
	log.Errorf(nerr.Error())
	return nerr
}

func (b LoghostBroker) Provision(_ context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {
	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	var ctx model.ContextProvision
	err = json.Unmarshal(details.RawContext, &ctx)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("error when loading context: %s", err.Error())
	}
	var params model.ProvisionParams
	err = json.Unmarshal(details.RawParameters, &params)
	if err != nil && len(details.RawParameters) > 0 {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("error when loading params: %s", err.Error())
	}

	// copy to not modify parent map
	tags := utils.CopyMapString(syslogAddr.Tags)
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
	if b.config.Broker.ForceEmptyDrainType {
		drainType = ""
	}
	patterns := append(syslogAddr.Patterns, params.Patterns...)

	// clean if something exists before
	err = b.db.Delete(model.Pattern{}, "instance_id = ?", instanceID).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return domain.ProvisionedServiceSpec{}, b.newDBError("provision", err)
	}
	err = b.db.Delete(model.Label{}, "instance_id = ?", instanceID).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return domain.ProvisionedServiceSpec{}, b.newDBError("provision", err)
	}
	err = b.db.Delete(model.SourceLabel{}, "instance_id = ?", instanceID).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return domain.ProvisionedServiceSpec{}, b.newDBError("provision", err)
	}

	err = b.db.Create(&model.InstanceParam{
		InstanceID:   instanceID,
		SpaceID:      ctx.SpaceGUID,
		OrgID:        ctx.OrganizationGUID,
		Namespace:    ctx.Namespace,
		SyslogName:   syslogAddr.Name,
		Patterns:     model.ListToPatterns(patterns),
		SourceLabels: model.MapToSourceLabels(utils.CopyMapString(syslogAddr.SourceLabels)),
		Tags:         model.MapToLabels(tags),
		CompanyID:    syslogAddr.CompanyID,
		UseTls:       params.UseTLS || b.config.HasTLS(),
		DrainType:    model.DrainType(strings.ToLower(string(drainType))),
		Revision:     1,
	}).Error
	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.newDBError("provision", err)
	}
	return domain.ProvisionedServiceSpec{
		DashboardURL: b.genDashboardURL(instanceID),
	}, nil
}

func (b LoghostBroker) Deprovision(
	ctx context.Context,
	instanceID string,
	details domain.DeprovisionDetails,
	asyncAllowed bool,
) (domain.DeprovisionServiceSpec, error) {

	err := b.db.Delete(&model.LogMetadata{
		InstanceID: instanceID,
	}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.DeprovisionServiceSpec{}, b.newDBError("deprovision", err)
	}

	err = b.db.Delete(&model.InstanceParam{
		InstanceID: instanceID,
	}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.DeprovisionServiceSpec{}, b.newDBError("deprovision", err)
	}

	err = b.db.Delete(&model.InstanceParam{}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.DeprovisionServiceSpec{}, b.newDBError("deprovision", err)
	}

	return domain.DeprovisionServiceSpec{}, nil
}

func (b LoghostBroker) Bind(
	_ context.Context,
	instanceID string,
	bindingID string,
	details domain.BindDetails,
	asyncAllowed bool,
) (domain.Binding, error) {

	var instanceParam model.InstanceParam
	var ctx model.ContextBind
	err := json.Unmarshal(details.RawContext, &ctx)
	if err != nil {
		return domain.Binding{}, fmt.Errorf("error when loading context: %s", err.Error())
	}

	err = b.db.Order("revision desc").First(&instanceParam, "instance_id = ?", instanceID).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return domain.Binding{}, fmt.Errorf("instance id '%s' not found", instanceID)
		}
		return domain.Binding{}, b.newDBError("bind", err)
	}

	appGuid := ctx.AppGUID
	if appGuid == "" {
		appGuid = details.AppGUID
	}

	err = b.db.Create(&model.LogMetadata{
		BindingID:  bindingID,
		InstanceID: instanceID,
		AppID:      appGuid,
	}).Error
	if err != nil {
		return domain.Binding{}, b.newDBError("bind", err)
	}

	syslogDrainURL := b.genURL(instanceParam, bindingID)
	return domain.Binding{
		SyslogDrainURL: syslogDrainURL,
	}, nil
}

func (b LoghostBroker) genDashboardURL(instanceID string) string {
	return fmt.Sprintf("https://%s/docs/%s", b.config.Broker.PublicHost, instanceID)
}

func (b LoghostBroker) genDocURL() string {
	return fmt.Sprintf("https://%s/docs", b.config.Broker.PublicHost)
}

func (b LoghostBroker) genURL(instanceParam model.InstanceParam, bindingID string) string {
	scheme := "http"
	port := b.config.Web.Port

	if instanceParam.UseTls && b.config.HasTLS() {
		scheme = "https"
		port = b.config.Web.TLS.Port
	}

	syslogDrainURL := fmt.Sprintf("%s://%s:%d/%s", scheme, b.config.Broker.DrainHost, port, bindingID)

	queryValues := make(url.Values)
	queryValues.Add(model.RevKey, fmt.Sprint(instanceParam.Revision))
	if instanceParam.DrainType != "" {
		queryValues.Add(model.DrainTypeKey, string(instanceParam.DrainType))
	}
	syslogDrainURL += fmt.Sprintf("?%s", queryValues.Encode())
	return syslogDrainURL
}

func (b LoghostBroker) Unbind(
	_ context.Context,
	instanceID string,
	bindingID string,
	details domain.UnbindDetails,
	asyncAllowed bool,
) (domain.UnbindSpec, error) {

	err := b.db.Delete(model.LogMetadata{}, "binding_id = ?", bindingID).Error
	if err != nil {
		return domain.UnbindSpec{}, b.newDBError("unbind", err)
	}
	return domain.UnbindSpec{}, nil
}

func (b LoghostBroker) Update(
	_ context.Context,
	instanceID string,
	details domain.UpdateDetails,
	asyncAllowed bool,
) (domain.UpdateServiceSpec, error) {

	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(details.PlanID)
	if err != nil {
		return domain.UpdateServiceSpec{}, err
	}

	var instanceParam model.InstanceParam
	err = b.db.Set("gorm:auto_preload", true).
		Order("revision desc").
		First(&instanceParam, "instance_id = ?", instanceID).
		Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return domain.UpdateServiceSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
		}
		return domain.UpdateServiceSpec{}, b.newDBError("update", err)
	}

	var ctx model.ContextProvision
	err = json.Unmarshal(details.RawContext, &ctx)
	if err != nil {
		return domain.UpdateServiceSpec{}, fmt.Errorf("error when loading context: %s", err.Error())
	}
	var params model.ProvisionParams
	err = json.Unmarshal(details.RawParameters, &params)
	if err != nil && len(details.RawParameters) > 0 {
		return domain.UpdateServiceSpec{}, fmt.Errorf("error when loading params: %s", err.Error())
	}

	// copy to not modify parent map
	tags := utils.CopyMapString(syslogAddr.Tags)
	if tags == nil {
		tags = make(map[string]string)
	}

	for k, v := range params.Tags {
		tags[k] = v
	}

	err = b.db.Delete(model.Pattern{}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.UpdateServiceSpec{}, b.newDBError("update", err)
	}
	err = b.db.Delete(model.Label{}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.UpdateServiceSpec{}, b.newDBError("update", err)
	}
	err = b.db.Delete(model.SourceLabel{}, "instance_id = ?", instanceID).Error
	if err != nil {
		return domain.UpdateServiceSpec{}, b.newDBError("update", err)
	}

	drainType := syslogAddr.DefaultDrainType
	if params.DrainType != nil && *params.DrainType != "" {
		drainType = *params.DrainType
	}
	if b.config.Broker.ForceEmptyDrainType {
		drainType = ""
	}
	patterns := append(syslogAddr.Patterns, params.Patterns...)
	err = b.db.Create(&model.InstanceParam{
		InstanceID:   instanceID,
		SpaceID:      instanceParam.SpaceID,
		OrgID:        instanceParam.OrgID,
		Namespace:    instanceParam.Namespace,
		SyslogName:   syslogAddr.Name,
		Patterns:     model.ListToPatterns(patterns),
		SourceLabels: model.MapToSourceLabels(utils.CopyMapString(syslogAddr.SourceLabels)),
		Tags:         model.MapToLabels(tags),
		CompanyID:    syslogAddr.CompanyID,
		UseTls:       b.config.HasTLS(),
		DrainType:    model.DrainType(strings.ToLower(string(drainType))),
		Revision:     instanceParam.Revision + 1,
	}).Error
	if err != nil {
		return domain.UpdateServiceSpec{}, b.newDBError("update", err)
	}

	return domain.UpdateServiceSpec{
		DashboardURL: b.genDashboardURL(instanceID),
	}, nil
}

func (LoghostBroker) LastOperation(
	ctx context.Context,
	instanceID string,
	details domain.PollDetails,
) (domain.LastOperation, error) {

	return domain.LastOperation{}, nil
}

func (b LoghostBroker) GetInstance(
	_ context.Context,
	instanceID string,
) (domain.GetInstanceDetailsSpec, error) {

	var instanceParam model.InstanceParam

	err := b.db.Set("gorm:auto_preload", true).
		Order("revision desc").
		First(&instanceParam, "instance_id = ?", instanceID).
		Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return domain.GetInstanceDetailsSpec{}, fmt.Errorf("instance id '%s' not found", instanceID)
		}
		return domain.GetInstanceDetailsSpec{}, b.newDBError("getinstance", err)
	}

	syslogAddr, err := model.SyslogAddresses(b.config.SyslogAddresses).FoundSyslogWriter(instanceParam.SyslogName)
	if err != nil {
		return domain.GetInstanceDetailsSpec{}, err
	}

	return domain.GetInstanceDetailsSpec{
		PlanID:       syslogAddr.ID,
		ServiceID:    serviceId,
		DashboardURL: b.genDashboardURL(instanceID),
		Parameters: model.ProvisionParams{
			Tags:     instanceParam.TagsToMap(),
			Patterns: model.Patterns(instanceParam.Patterns).ToList(),
		},
	}, nil
}

func (b LoghostBroker) GetBinding(
	_ context.Context,
	instanceID string,
	bindingID string,
) (domain.GetBindingSpec, error) {

	var logData model.LogMetadata

	err := b.db.First(&logData, "binding_id = ?", bindingID).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return domain.GetBindingSpec{}, fmt.Errorf("binding id '%s' not found", bindingID)
		}
		return domain.GetBindingSpec{}, b.newDBError("get-binding", err)
	}

	var instanceParam model.InstanceParam
	err = b.db.Set("gorm:auto_preload", true).
		Order("revision desc").
		First(&instanceParam, "instance_id = ?", logData.InstanceID).
		Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return domain.GetBindingSpec{}, fmt.Errorf("instance id '%s' not found", logData.InstanceID)
		}
		return domain.GetBindingSpec{}, b.newDBError("get-binding", err)
	}
	logData.InstanceParam = instanceParam
	syslogDrainURL := b.genURL(instanceParam, bindingID)
	return domain.GetBindingSpec{
		SyslogDrainURL: syslogDrainURL,
	}, nil
}

func (b LoghostBroker) LastBindingOperation(
	ctx context.Context,
	instanceID string,
	bindingID string,
	details domain.PollDetails,
) (domain.LastOperation, error) {

	return domain.LastOperation{}, nil
}
