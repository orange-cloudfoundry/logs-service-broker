package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-community/gautocloud"
	"github.com/cloudfoundry-community/gautocloud/connectors/generic"
	"github.com/jinzhu/gorm"
	"github.com/pivotal-cf/brokerapi"
)

func init() {
	gautocloud.RegisterConnector(generic.NewConfigGenericConnector(Config{}))
}

const (
	PlatformCF   = "cloudfoundry"
	PlatformK8s  = "kubernetes"
	RevKey       = "rev"
	EndOfLifeKey = "end-of-life"
	DrainTypeKey = "drain-type"
)

type LogConfig struct {
	Level          string `cloud:"level"`
	JSON           *bool  `cloud:"json"`
	NoColor        bool   `cloud:"no_color"`
	EnableProfiler bool   `cloud:"enable_profiler"`
}

type WebTLSConfig struct {
	Port     int    `cloud:"port"`
	CertFile string `cloud:"cert_file" cloud-default:""`
	KeyFile  string `cloud:"key_file"  cloud-default:""`
}

type KeepAliveConfig struct {
	Disabled  bool   `cloud:"disabled"`
	Duration  string `cloud:"duration" cloud-default:"4m"`
	Fuzziness string `cloud:"fuzziness" cloud-default:"1m"`
	duration  *time.Duration
	fuzziness *time.Duration
}

type WebConfig struct {
	Port                 int             `cloud:"port"`
	MaxKeepAlive         KeepAliveConfig `cloud:"max_keep_alive"`
	TLS                  WebTLSConfig    `cloud:"tls"`
	maxKeepAliveDuration *time.Duration
}

// GetPort - Compute port according to gautocloud and configuration
func (w *WebConfig) GetPort() int {
	port := w.Port
	if gautocloud.GetAppInfo().Port > 0 {
		port = gautocloud.GetAppInfo().Port
	}
	if port == 0 {
		port = 8088
	}
	return port
}

type BrokerConfig struct {
	PublicHost          string `cloud:"public_host"`
	DrainHost           string `cloud:"drain_host"`
	Username            string `cloud:"username"`
	Password            string `cloud:"password"`
	ForceEmptyDrainType bool   `cloud:"force_empty_drain_type"`
	VirtualHost         bool   `cloud:"virtual_host"`
}

type DBConfig struct {
	CnxMaxIdle     int    `cloud:"cnx_max_idle" cloud-default:"20"`
	CnxMaxOpen     int    `cloud:"cnx_max_open" cloud-default:"100"`
	CnxMaxLife     string `cloud:"cnx_max_life" cloud-default:"1h"`
	SQLiteFallback bool   `cloud:"sqlite_fallback"`
	SQLitePath     string `cloud:"sqlite_path" cloud-default:"loghostsvc.db"`
}

type BindingCacheConfig struct {
	Duration string `cloud:"duration" cloud-default:"10m"`
}

type ForwarderConfig struct {
	AllowedHosts []string     `cloud:"allowed_hosts"`
	ParsingKeys  []ParsingKey `cloud:"parsing_keys"`
}

func (f *KeepAliveConfig) GetDuration() *time.Duration {
	if f.duration == nil {
		dur, err := time.ParseDuration(f.Duration)
		if err != nil {
			dur, _ = time.ParseDuration("4m")
		}
		f.duration = &dur
	}
	return f.duration
}

func (f *KeepAliveConfig) GetFuzziness() *time.Duration {
	if f.fuzziness == nil {
		dur, err := time.ParseDuration(f.Fuzziness)
		if err != nil {
			dur, _ = time.ParseDuration("4m")
		}
		f.fuzziness = &dur
	}
	return f.fuzziness
}

type Config struct {
	Web             WebConfig          `cloud:"web"`
	Log             LogConfig          `cloud:"log"`
	SyslogAddresses []SyslogAddress    `cloud:"syslog_addresses"`
	Broker          BrokerConfig       `cloud:"broker"`
	DB              DBConfig           `cloud:"db"`
	Forwarder       ForwarderConfig    `cloud:"forwarder"`
	BindingCache    BindingCacheConfig `cloud:"binding_cache"`
}

func (c Config) HasTLS() bool {
	return c.Web.TLS.CertFile != "" && c.Web.TLS.KeyFile != "" && c.Web.TLS.Port > 0
}

type ParsingKey struct {
	Name string `cloud:"name"`
	Hide bool   `cloud:"hide"`
}

type SyslogAddresses []SyslogAddress

func (a SyslogAddresses) ToServicePlans() []brokerapi.ServicePlan {
	sp := make([]brokerapi.ServicePlan, len(a))
	for i, sa := range a {
		sp[i] = sa.ToServicePlan()
	}
	return sp
}

func (a SyslogAddresses) FoundSyslogWriter(planIDOrName string) (SyslogAddress, error) {
	for _, addr := range a {
		if addr.ID == planIDOrName || addr.Name == planIDOrName {
			return addr, nil
		}
	}
	return SyslogAddress{}, fmt.Errorf("Cannot found syslog writer for plan id or name '%s'.", planIDOrName)
}

type SyslogAddress struct {
	ID               string            `cloud:"id"`
	Name             string            `cloud:"name"`
	CompanyID        string            `cloud:"company_id"`
	Description      string            `cloud:"description"`
	DefaultDrainType DrainType         `cloud:"default_drain_type"`
	URLs             []string          `cloud:"urls"`
	Bullets          []string          `cloud:"bullets"`
	Patterns         []string          `cloud:"patterns"`
	Tags             map[string]string `cloud:"tags"`
	SourceLabels     map[string]string `cloud:"source_labels"`
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
	InstanceID   string `gorm:"primary_key"`
	Revision     int    `gorm:"primary_key;auto_increment:false"`
	SpaceID      string
	OrgID        string
	Namespace    string
	SyslogName   string
	CompanyID    string
	UseTls       bool
	DrainType    DrainType
	Patterns     []Pattern     `gorm:"foreignkey:InstanceID"`
	Tags         []Label       `gorm:"foreignkey:InstanceID"`
	SourceLabels []SourceLabel `gorm:"foreignkey:InstanceID"`
}

func (d *InstanceParam) TagsToMap() map[string]string {
	m := make(map[string]string)
	for _, label := range d.Tags {
		m[label.Key] = label.Value
	}
	return m
}

func (d *InstanceParam) BeforeDelete(tx *gorm.DB) (err error) {
	if d.InstanceID == "" {
		return nil
	}
	tx.Delete(Pattern{}, "instance_id = ?", d.InstanceID)
	tx.Delete(Label{}, "instance_id = ?", d.InstanceID)
	tx.Delete(SourceLabel{}, "instance_id = ?", d.InstanceID)
	return nil
}

type LogMetadata struct {
	BindingID     string        `gorm:"primary_key"`
	InstanceParam InstanceParam `gorm:"foreignkey:InstanceID;association_foreignkey:InstanceID"`
	InstanceID    string
	AppID         string
}

type Label struct {
	ID         uint `gorm:"primary_key;auto_increment"`
	Key        string
	Value      string `gorm:"size:600"`
	InstanceID string
}

type SourceLabel struct {
	ID         uint `gorm:"primary_key;auto_increment"`
	Key        string
	Value      string `gorm:"size:600"`
	InstanceID string
}

type SourceLabels []SourceLabel

func (labels SourceLabels) ToMap() map[string]string {
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
	Patterns  []string          `json:"patterns"`
	Tags      map[string]string `json:"tags"`
	UseTLS    bool              `json:"use_tls"`
	DrainType *DrainType        `json:"drain_type"`
}

type DrainType string

func (dt *DrainType) UnmarshalJSON(b []byte) error {
	var dtStr string
	err := json.Unmarshal(b, &dtStr)
	if err != nil {
		return err
	}
	dtStr = strings.ToLower(dtStr)
	*dt = DrainType(dtStr)
	switch dtStr {
	case "":
		return nil
	case "logs":
		return nil
	case "metrics":
		return nil
	case "all":
		return nil
	default:
		return fmt.Errorf("Only drain_type `metrics` or `logs` or `all` or empty value is allowed (which means only logs)")
	}
	return nil
}

type Patterns []Pattern

func (p Patterns) ToList() []string {
	ls := make([]string, len(p))
	for i, pattern := range p {
		ls[i] = pattern.Pattern
	}
	return ls
}

func ListToPatterns(l []string) []Pattern {
	if l == nil || len(l) == 0 {
		return []Pattern{}
	}
	patterns := make([]Pattern, len(l))
	i := 0
	for _, v := range l {
		patterns[i] = Pattern{
			Pattern: v,
		}
		i++
	}
	return patterns
}

func MapToLabels(m map[string]string) []Label {
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

func MapToSourceLabels(m map[string]string) []SourceLabel {
	if m == nil || len(m) == 0 {
		return []SourceLabel{}
	}
	tags := make([]SourceLabel, len(m))
	i := 0
	for k, v := range m {
		tags[i] = SourceLabel{
			Key:   k,
			Value: v,
		}
		i++
	}
	return tags
}
