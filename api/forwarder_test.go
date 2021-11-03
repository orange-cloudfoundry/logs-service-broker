package api_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/orange-cloudfoundry/logs-service-broker/api"
	"github.com/orange-cloudfoundry/logs-service-broker/api/fakes"
	"github.com/orange-cloudfoundry/logs-service-broker/dbservices"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

var _ = Describe("Forwarder", func() {

	var forwarder *api.Forwarder
	var db *gorm.DB
	var err error
	var writers map[string]io.WriteCloser
	var serviceID = "ad45d7cc-4795-4554"
	var bindingID = "125ce4a5-7845-14ae"

	BeforeEach(func() {
		//init BDD
		db, err = gorm.Open("sqlite3", "file:forwarderdb?mode=memory&cache=shared")
		Expect(err).ShouldNot(HaveOccurred())
		db.AutoMigrate(
			&model.LogMetadata{},
			&model.InstanceParam{},
			&model.Patterns{},
			&model.Label{},
			&model.SourceLabel{},
		)

		result := db.Create(&model.InstanceParam{
			InstanceID: serviceID,
			Revision:   4,
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

		// init forwarder
		var cacher, err = dbservices.NewMetaCacher(db, "5m")
		Expect(err).ToNot(HaveOccurred())

		writers = make(map[string]io.WriteCloser)
		writers["loghost"] = fakes.NewFakeWriter()
		forwarder = api.NewForwarder(cacher, writers, &model.Config{
			Forwarder: model.ForwarderConfig{
				AllowedHosts: []string{
					"logservice.private.domain",
				},
				ParsingKeys: []model.ParsingKey{
					{
						Name: "app.audit_data.messages.last",
						Hide: true,
					},
				},
			},
		})
	})

	AfterEach(func() {
		// shutdown all servers
		db.Exec("DELETE FROM log_metadata;")
		db.Exec("DELETE FROM instance_params;")
		db.Close()
	})

	Context("When a message is received", func() {

		It("formards the message", func() {
			var message = `<14>1 2006-01-02T15:04:05.999999Z org.space.app - [metrics] - [timer@47450 name="my-timer" start="0" stop="10"] - app.hbx.geo.francetelecom.fr:443`
			var forwardedMessage = `<14>1 2006-01-02T15:04:05.999999Z org.space.app - [metrics] - [logsbroker@1368 app="org/space/app" app_id="3" app_name="app" org="org" org_id="1" space="space" space_id="2"] {"@cf":{"app":"app","app_id":"3","app_instance":0,"org":"org","org_id":"1","space":"space","space_id":"2"},"@input":"syslog","@level":"INFO","@metric":{"name":"my-timer","start":0,"stop":10,"type":"timer"},"@shipper":{"name":"log-service","priority":14},"@source":{"details":"","type":"metrics"},"@timestamp":"2006-01-02T15:04:05.999999Z","@type":"Metrics"}
`
			req, err := http.NewRequest("GET", fmt.Sprintf("/%s?rev=4", bindingID), bytes.NewBufferString(message))
			req.Host = "logservice.private.domain:8089"
			Expect(err).ToNot(HaveOccurred())
			r := mux.NewRouter()
			rr := httptest.NewRecorder()
			r.HandleFunc("/{bindingId}", forwarder.ServeHTTP)
			r.ServeHTTP(rr, req)

			// waiting for message forwarding
			time.Sleep(time.Millisecond * 10)

			Expect(rr.Code).To(Equal(http.StatusOK))
			Expect(writers["loghost"]).ToNot(BeNil())
			Expect(*(writers["loghost"].(*fakes.FakeWriter).GetBuffer())).To(Equal(forwardedMessage))
		})
	})
})
