package model

import (
	"github.com/cloudfoundry-community/gautocloud"
	"github.com/cloudfoundry-community/gautocloud/connectors/generic"
	"github.com/jinzhu/gorm"
	"github.com/pivotal-cf/brokerapi"
)

func init() {
	gautocloud.RegisterConnector(generic.NewConfigGenericConnector(Config{}))
}

const (
	PlatformCF  = "cloudfoundry"
	PlatformK8s = "kubernetes"
)

type Config struct {
	SyslogAddresses []SyslogAddress `cloud:"syslog_addresses"`
	BrokerUsername  string          `cloud:"broker_username"`
	BrokerPassword  string          `cloud:"broker_password"`
	SyslogDrainURL  string          `cloud:"syslog_drain_url"`
	VirtualHost     bool            `cloud:"virtual_host"`
	LogLevel        string          `cloud:"log_level"`
	LogJSON         *bool           `cloud:"log_json"`
	LogNoColor      bool            `cloud:"log_no_color"`
	SSLCertFile     string          `cloud:"ssl_cert_file" cloud-default:""`
	SSLKeyFile      string          `cloud:"ssl_key_file" cloud-default:""`
	SQLitePath      string          `cloud:"sqlite_path" cloud-default:"loghostsvc.db"`
	SQLCnxMaxIdle   int             `cloud:"sql_cnx_max_idle" cloud-default:"20"`
	SQLCnxMaxOpen   int             `cloud:"sql_cnx_max_open" cloud-default:"100"`
	SQLCnxMaxLife   string          `cloud:"sql_cnx_max_life" cloud-default:"1h"`
}

type SyslogAddresses []SyslogAddress

func (a SyslogAddresses) ToServicePlans() []brokerapi.ServicePlan {
	sp := make([]brokerapi.ServicePlan, len(a))
	for i, sa := range a {
		sp[i] = sa.ToServicePlan()
	}
	return sp
}

type SyslogAddress struct {
	ID           string            `cloud:"id"`
	CompanyID    string            `cloud:"company_id"`
	Name         string            `cloud:"name"`
	Description  string            `cloud:"description"`
	Bullets      []string          `cloud:"bullets"`
	URLs         []string          `cloud:"urls"`
	Tags         map[string]string `cloud:"tags"`
	SourceLabels map[string]string `cloud:"source_labels"`
}

func (a SyslogAddress) ToServicePlan() brokerapi.ServicePlan {
	return brokerapi.ServicePlan{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Metadata: &brokerapi.ServicePlanMetadata{
			Bullets:     a.Bullets,
			DisplayName: a.Name,
		},
	}
}

type InstanceParam struct {
	InstanceID string `gorm:"primary_key"`
	SpaceID    string
	OrgID      string
	Namespace  string
	SyslogName string
	CompanyID  string
	Patterns   []Pattern `gorm:"foreignkey:InstanceID"`
	Tags       []Label   `gorm:"foreignkey:InstanceID"`
}

func (d *InstanceParam) BeforeDelete(tx *gorm.DB) (err error) {
	tx.Delete(Pattern{}, "instance_id = ?", d.InstanceID)
	tx.Delete(Label{}, "instance_id = ?", d.InstanceID)

	return
}

type LogMetadata struct {
	BindingID     string        `gorm:"primary_key"`
	InstanceParam InstanceParam `gorm:"foreignkey:InstanceID;association_foreignkey:InstanceID"`
	InstanceID    string
	AppID         string
	Patterns      []Pattern `gorm:"foreignkey:BindingID"`
	Tags          []Label   `gorm:"foreignkey:BindingID"`
	SourceLabels  []Label   `gorm:"foreignkey:BindingID"`
}

func (d *LogMetadata) BeforeDelete(tx *gorm.DB) (err error) {
	tx.Delete(Pattern{}, "binding_id = ?", d.BindingID)
	tx.Delete(Label{}, "binding_id = ?", d.BindingID)
	return
}

type Label struct {
	ID         uint `gorm:"primary_key;auto_increment"`
	Key        string
	Value      string `gorm:"size:600"`
	InstanceID string
	BindingID  string
}

type Labels []Label

func (labels Labels) ToMap() map[string]string {
	m := make(map[string]string)
	for _, label := range labels {
		m[label.Key] = label.Value
	}
	return m
}

type Pattern struct {
	ID         uint   `gorm:"primary_key;auto_increment"`
	Pattern    string `gorm:"size:2550"`
	InstanceID string
	BindingID  string
}

type ContextProvision struct {
	ContextCF
	ContextK8S
	Platform string `json:"platform"`
}

func (c ContextProvision) IsCloudFoundry() bool {
	return c.Platform == PlatformCF
}

func (c ContextProvision) IsKubernetes() bool {
	return c.Platform == PlatformK8s
}

type ContextCF struct {
	OrganizationGUID string `json:"organization_guid,omitempty"`
	SpaceGUID        string `json:"space_guid,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	ServiceGUID      string `json:"service_guid,omitempty"`
}

type ContextK8S struct {
	Namespace string `json:"namespace,omitempty"`
	ClusterID string `json:"clusterid,omitempty"`
}

type ContextBind struct {
	AppGUID string `json:"app_guid"`
}

type CfResponse struct {
	Entity struct {
		Name string `json:"name"`
	} `json:"entity"`
}

type ProvisionParams struct {
	Patterns []string          `json:"patterns"`
	Tags     map[string]string `json:"tags"`
}

type BindingParams struct {
	Patterns []string          `json:"patterns"`
	Tags     map[string]string `json:"tags"`
}

type Patterns []Pattern

func (p Patterns) ToList() []string {
	ls := make([]string, len(p))
	for i, pattern := range p {
		ls[i] = pattern.Pattern
	}
	return ls
}

func MapToTags(m map[string]string) []Label {
	if m == nil || len(m) == 0 {
		return []Label{}
	}
	tags := make([]Label, len(m))
	i := 0
	for k, v := range m {
		tags[i] = Label{
			Key:   k,
			Value: v,
		}
		i++
	}
	return tags
}
