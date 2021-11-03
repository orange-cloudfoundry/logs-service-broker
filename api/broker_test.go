package api_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/pivotal-cf/brokerapi"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"

	"github.com/orange-cloudfoundry/logs-service-broker/api"
	"github.com/orange-cloudfoundry/logs-service-broker/dbservices"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

var _ = Describe("Broker", func() {
	//var db *gorm.DB
	//var err error
	//var jilaServer *ghttp.Server
	var broker *api.LoghostBroker
	//var dashboardUrl = "http://localhost/dasboard"
	var planID = "1516-4544-5454"

	var tags = make(map[string]string)
	var config model.Config
	var sourceLabels = make(map[string]string)

	BeforeEach(func() {
		tags["app"] = "{{ .App }}{{with ( ret .Logdata \"@app.tags\" ) }}/{{.}}{{end}}"
		tags["env"] = "{{if hasSuffix .Org \"-staging\" }}dev{{ else }}prod{{ end }}"
		tags["audience"] = "mydept"
		tags["fmt"] = "json"
		tags["s"] = "cloudfoundry"

		sourceLabels["deployment"] = "production"

		config = model.Config{
			Broker: model.BrokerConfig{
				PublicHost:          "logservice.public.domain",
				DrainHost:           "logservice.private.domain",
				Username:            "username",
				Password:            "xxxxxx",
				ForceEmptyDrainType: true,
			},
			SyslogAddresses: []model.SyslogAddress{
				{
					ID:               planID,
					Name:             "loghost",
					CompanyID:        "logsbroker@1368",
					Description:      "Drain apps logs to loghost",
					DefaultDrainType: "all",
					URLs: []string{
						"tcp://elk-collector.private.domain:1514",
					},
					Bullets: []string{
						"Available create parameters:",
						"- s",
						"- env",
						"- audience",
					},
					Patterns: []string{
						//"%{TIME} %{WORD:[@app][program]}%{SPACE}\\|%{SPACE}%{NOTSPACE:[@app][tags]}.json%{SPACE}%{GREEDYDATA:@message}",
						//"%{TIME} %{WORD:[@app][program]}%{SPACE}\\|%{SPACE}%{NOTSPACE:[@app][tags]}.txt%{SPACE}%{GREEDYDATA:[text]}",
						//"%{NOTSPACE:[@app][tags]}.json%{SPACE}%{GREEDYDATA:@message}",
						//"%{NOTSPACE:[@app][tags]}.txt%{SPACE}%{GREEDYDATA:[text]}",
						"%{MODSECAPACHEERROR}%{GREEDYDATA:@message}",
						//"%{MODSECRULEMSG}",
					},
					Tags:         tags,
					SourceLabels: sourceLabels,
				},
			},
		}

	})

	Context("Services()", func() {
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "service", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			db.Close()
		})

		It("returns service description ", func() {
			services, err := broker.Services(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(len(services)).To(Equal(1))
			Expect(services[0].Name).To(Equal("logs"))
			Expect(services[0].Metadata.DisplayName).To(Equal("logs"))
			Expect(services[0].Metadata.DocumentationUrl).To(Equal("https://logservice.public.domain/docs"))
			Expect(len(services[0].Plans)).To(Equal(1))
			Expect(services[0].Plans[0].Name).To(Equal("loghost"))
		})

	})

	Context("Provision()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var specs brokerapi.ProvisionedServiceSpec
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "provision", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			db.Close()
		})

		When("all parameters are ok", func() {

			BeforeEach(func() {
				details := brokerapi.ProvisionDetails{
					ServiceID:     "11c147f0-297f-4fd6-9401-e94e64f37094",
					PlanID:        planID,
					RawContext:    []byte(`{"organization_guid": "1", "space_guid": "2", "service_guid": "11c147f0-297f-4fd6-9401-e94e64f37094", "plateform": "cloudfoundry"}`),
					RawParameters: []byte(`{"use_tls": false, "drain_type": "all", "patterns": ["%{MODSECRULEMSG}"], "tags": {"my-tag": "bar"}}`),
				}
				specs, err = broker.Provision(context.Background(), serviceID, details, true)
				Expect(err).ToNot(HaveOccurred())
			})

			It("inserts service into DB", func() {
				var (
					inst    model.InstanceParam
					pattern model.Pattern
					label   model.Label
					source  model.SourceLabel
				)

				db.First(&inst, "instance_id = ?", serviceID)
				Expect(inst.OrgID).To(Equal("1"))
				Expect(inst.SpaceID).To(Equal("2"))
				Expect(inst.Namespace).To(Equal(""))
				Expect(inst.SyslogName).To(Equal("loghost"))

				db.Last(&pattern, "instance_id = ?", serviceID)
				Expect(pattern.ID).To(Equal(uint(2)))
				Expect(pattern.Pattern).To(Equal("%{MODSECRULEMSG}"))

				db.First(&label, "instance_id = ? and key = ?", serviceID, "my-tag")
				Expect(label.Value).To(Equal("bar"))

				db.First(&source, "instance_id = ?", serviceID)
				Expect(source.Key).To(Equal("deployment"))
				Expect(source.Value).To(Equal("production"))
			})

			It("returns dashboard url", func() {
				Expect(specs.DashboardURL).To(Equal("https://logservice.public.domain/docs/ad45d7cc-4795-4554"))
			})
		})
	})

	Context("Deprovision()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "deprovision", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())

			result := db.Create(&model.InstanceParam{
				InstanceID: serviceID,
				Revision:   1,
				OrgID:      "1",
				SpaceID:    "2",
				SyslogName: "loghost",
			})
			Expect(result.Error).To(BeNil())
			result2 := db.Create(&model.Pattern{
				InstanceID: serviceID,
				Pattern:    "my-pattern",
			})
			Expect(result2.Error).To(BeNil())
			result3 := db.Create(&model.Label{
				Key:   "my-key",
				Value: "my-value",
			})
			Expect(result3.Error).To(BeNil())
			result4 := db.Create(&model.SourceLabel{
				Key:   "my-key",
				Value: "my-value",
			})
			Expect(result4.Error).To(BeNil())
		})

		AfterEach(func() {
			db.Close()
		})

		It("removes data from DB", func() {
			var (
				inst    model.InstanceParam
				pattern model.Pattern
				label   model.Label
				source  model.SourceLabel
			)
			// check if database is Ok before the deprovision
			db.First(&inst, "instance_id = ?", serviceID)
			Expect(inst).NotTo(Equal((model.InstanceParam{})))
			inst = model.InstanceParam{}

			details := brokerapi.DeprovisionDetails{
				ServiceID: "11c147f0-297f-4fd6-9401-e94e64f37094",
				PlanID:    planID,
				Force:     true,
			}
			_, err = broker.Deprovision(context.Background(), serviceID, details, false)
			Expect(err).ToNot(HaveOccurred())

			db.First(&inst, "instance_id = ?", serviceID)
			Expect(inst).To(Equal((model.InstanceParam{})))
			db.First(&pattern, "instance_id = ?", serviceID)
			Expect(pattern).To(Equal((model.Pattern{})))
			db.First(&label, "instance_id = ?", serviceID)
			Expect(label).To(Equal((model.Label{})))
			db.First(&source, "instance_id = ?", serviceID)
			Expect(source).To(Equal((model.SourceLabel{})))
		})
	})

	Context("Bind()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var bindingID = "125ce4a5-7845-14ae"
		var specs brokerapi.Binding
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "bind", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			db.Close()
		})

		When("all parameters are ok", func() {

			BeforeEach(func() {
				result := db.Create(&model.InstanceParam{
					InstanceID: serviceID,
					Revision:   1,
					OrgID:      "1",
					SpaceID:    "2",
					SyslogName: "loghost",
				})
				Expect(result.Error).To(BeNil())
				details := brokerapi.BindDetails{
					ServiceID:  "11c147f0-297f-4fd6-9401-e94e64f37094",
					PlanID:     planID,
					AppGUID:    "3",
					RawContext: []byte(`{"app_guid": "3"}`),
				}
				specs, err = broker.Bind(context.Background(), serviceID, bindingID, details, false)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				db.Exec("DELETE FROM instance_params;")
			})

			It("inserts service into DB", func() {
				var (
					metadata model.LogMetadata
				)
				db.First(&metadata, "binding_id = ?", bindingID)
				Expect(metadata.AppID).To(Equal("3"))
				Expect(metadata.InstanceID).To(Equal(serviceID))
			})

			It("returns syslog drain url", func() {
				Expect(specs.SyslogDrainURL).To(Equal("http://logservice.private.domain:0/125ce4a5-7845-14ae?rev=1"))
			})
		})

	})

	Context("UnBind()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var bindingID = "125ce4a5-7845-14ae"

		var db *gorm.DB
		var err error
		BeforeEach(func() {
			err = initBroker(&db, "unbind", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())

			result := db.Create(&model.InstanceParam{
				InstanceID: serviceID,
				Revision:   1,
				OrgID:      "1",
				SpaceID:    "2",
				SyslogName: "loghost",
			})
			Expect(result.Error).To(BeNil())
			result2 := db.Create(&model.LogMetadata{
				InstanceID: serviceID,
				BindingID:  bindingID,
				AppID:      "3",
			})
			Expect(result2.Error).To(BeNil())
		})

		AfterEach(func() {
			db.Close()
		})

		It("removes data from DB", func() {

			var (
				metadata model.LogMetadata
			)

			// check if database is Ok before the deprovision
			db.First(&metadata, "binding_id = ?", bindingID)
			Expect(metadata).NotTo(Equal((model.LogMetadata{})))
			metadata = model.LogMetadata{}

			details := brokerapi.UnbindDetails{
				ServiceID: "11c147f0-297f-4fd6-9401-e94e64f37094",
				PlanID:    planID,
			}
			_, err = broker.Unbind(context.Background(), serviceID, bindingID, details, false)
			Expect(err).ToNot(HaveOccurred())

			db.First(&metadata, "binding_id = ?", bindingID)
			Expect(metadata).To(Equal((model.LogMetadata{})))
		})
	})

	Context("Update()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var bindingID = "125ce4a5-7845-14ae"
		var specs brokerapi.UpdateServiceSpec
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "update", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())

			// create informations in database
			result := db.Create(&model.InstanceParam{
				InstanceID: serviceID,
				Revision:   1,
				OrgID:      "1",
				SpaceID:    "2",
				SyslogName: "loghost",
				CompanyID:  "logsbroker@1368",
				UseTls:     true,
				DrainType:  "all",
			})
			Expect(result.Error).To(BeNil())
			result2 := db.Create(&model.LogMetadata{
				BindingID:  bindingID,
				InstanceID: serviceID,
				AppID:      "3",
			})
			Expect(result2.Error).To(BeNil())
			result3 := db.Create(&model.Pattern{
				ID:         uint(1),
				InstanceID: serviceID,
				Pattern:    "my-pattern",
			})
			Expect(result3.Error).To(BeNil())
			result4 := db.Create(&model.Label{
				ID:         uint(1),
				InstanceID: serviceID,
				Key:        "my-key",
				Value:      "my-value",
			})
			Expect(result4.Error).To(BeNil())
			result5 := db.Create(&model.SourceLabel{
				ID:         uint(1),
				InstanceID: serviceID,
				Key:        "other-key",
				Value:      "other-value",
			})
			Expect(result5.Error).To(BeNil())

			details := brokerapi.UpdateDetails{
				ServiceID:     "11c147f0-297f-4fd6-9401-e94e64f37094",
				PlanID:        planID,
				RawContext:    []byte(`{"organization_guid": "1", "space_guid": "2", "service_guid": "11c147f0-297f-4fd6-9401-e94e64f37094", "plateform": "cloudfoundry"}`),
				RawParameters: []byte(`{"use_tls": false, "drain_type": "all", "patterns": ["%{MODSECRULEMSG}"], "tags": {"my-tag": "bar"}}`),
			}

			specs, err = broker.Update(context.Background(), serviceID, details, true)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			db.Close()
		})

		When("all parameters are ok", func() {

			It("inserts data into DB", func() {
				var (
					inst    model.InstanceParam
					pattern model.Pattern
					label   model.Label
					source  model.SourceLabel
				)
				db.Order("revision desc").First(&inst, "instance_id = ?", serviceID)
				Expect(inst.OrgID).To(Equal("1"))
				Expect(inst.SpaceID).To(Equal("2"))
				Expect(inst.Namespace).To(Equal(""))
				Expect(inst.SyslogName).To(Equal("loghost"))
				Expect(inst.Revision).To(Equal(2))
				Expect(inst.UseTls).To(BeFalse())

				db.Last(&pattern, "instance_id = ?", serviceID)
				Expect(pattern.ID).To(Equal(uint(3)))
				Expect(pattern.Pattern).To(Equal("%{MODSECRULEMSG}"))

				db.First(&label, "instance_id = ? and key = ?", serviceID, "my-tag")
				Expect(label.Value).To(Equal("bar"))

				db.First(&source, "instance_id = ?", serviceID)
				Expect(source.Key).To(Equal("deployment"))
				Expect(source.Value).To(Equal("production"))

				Expect(specs.DashboardURL).To(Equal("https://logservice.public.domain/docs/ad45d7cc-4795-4554"))
			})
		})
	})

	Context("GetInstance()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "getinstance", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())

			result := db.Create(&model.InstanceParam{
				InstanceID: serviceID,
				Revision:   1,
				OrgID:      "1",
				SpaceID:    "2",
				SyslogName: "loghost",
			})
			Expect(result.Error).To(BeNil())
		})

		AfterEach(func() {
			db.Close()
		})

		It("returns instance data from DB", func() {
			specs, err := broker.GetInstance(context.Background(), serviceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(specs.PlanID).To(Equal(planID))
			Expect(specs.DashboardURL).To(Equal("https://logservice.public.domain/docs/ad45d7cc-4795-4554"))
		})
	})

	Context("GetBinding()", func() {
		var serviceID = "ad45d7cc-4795-4554"
		var bindingID = "125ce4a5-7845-14ae"
		var db *gorm.DB
		var err error

		BeforeEach(func() {
			err = initBroker(&db, "getBinding", &broker, &config)
			Expect(err).ShouldNot(HaveOccurred())

			result := db.Create(&model.InstanceParam{
				InstanceID: serviceID,
				Revision:   1,
				OrgID:      "1",
				SpaceID:    "2",
				SyslogName: "loghost",
			})
			Expect(result.Error).To(BeNil())
			result2 := db.Create(&model.LogMetadata{
				InstanceID: serviceID,
				BindingID:  bindingID,
				AppID:      "3",
			})
			Expect(result2.Error).To(BeNil())
		})

		AfterEach(func() {
			db.Close()
		})

		It("returns instance data from DB", func() {
			specs, err := broker.GetBinding(context.Background(), serviceID, bindingID)
			Expect(err).ToNot(HaveOccurred())
			Expect(specs.SyslogDrainURL).To(Equal("http://logservice.private.domain:0/125ce4a5-7845-14ae?rev=1"))
		})
	})
})

func initBroker(db **gorm.DB, name string, broker **api.LoghostBroker, config *model.Config) error {
	var err error
	*db, err = gorm.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	if err != nil {
		return err
	}
	(*db).AutoMigrate(
		&model.LogMetadata{},
		&model.InstanceParam{},
		&model.Patterns{},
		&model.Label{},
		&model.SourceLabel{},
	)

	cacher, err := dbservices.NewMetaCacher(*db, "5m")
	if err != nil {
		return err
	}
	*broker = api.NewLoghostBroker(*db, cacher, config)

	return nil
}
