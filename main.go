package main

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/cloudfoundry-community/gautocloud"
	_ "github.com/cloudfoundry-community/gautocloud/connectors/databases/gorm"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/syslog"
	"github.com/pivotal-cf/brokerapi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
)

func init() {
	if gautocloud.IsInACloudEnv() && gautocloud.CurrentCloudEnv().Name() != "localcloud" {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func main() {
	panic(boot())
}

func retrieveGormDb(sqlitePath string) *gorm.DB {
	var db *gorm.DB
	err := gautocloud.Inject(&db)
	if err == nil {
		return db
	}
	log.Warnf("Error when loading database, switching to sqlite, see message: %s", err.Error())
	db, _ = gorm.Open("sqlite3", sqlitePath)
	return db
}

func CreateWriters(sysAddresses []model.SyslogAddress) (map[string]io.WriteCloser, error) {
	writers := make(map[string]io.WriteCloser)
	for _, sysAddr := range sysAddresses {
		writer, err := syslog.NewWriter(sysAddr.URLs...)
		if err != nil {
			return writers, err
		}
		writers[sysAddr.Name] = writer
	}
	return writers, nil
}

func boot() error {
	var config model.Config
	gautocloud.Inject(&config)

	loadLogConfig(config)

	db := retrieveGormDb(config.SqlitePath)
	defer db.Close()

	db.AutoMigrate(&model.LogMetadata{}, &model.InstanceParam{}, &model.Patterns{}, &model.Tag{})

	sw, err := CreateWriters(config.SyslogAddresses)
	if err != nil {
		return err
	}
	defer func() {
		for _, w := range sw {
			w.Close()
		}
	}()

	f := NewForwarder(db, sw)
	broker := NewLoghostBroker(db, config)

	lag := lager.NewLogger("broker")
	lag.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerHandler := brokerapi.New(broker, lag, brokerapi.BrokerCredentials{
		Username: config.BrokerUsername,
		Password: config.BrokerPassword,
	})

	r := mux.NewRouter()
	r.NewRoute().MatcherFunc(func(req *http.Request, m *mux.RouteMatch) bool {
		return strings.HasPrefix(req.URL.Path, "/v2")
	}).Handler(brokerHandler)
	r.Handle("/metrics", promhttp.Handler())

	if config.VirtualHost {
		r.NewRoute().MatcherFunc(func(req *http.Request, m *mux.RouteMatch) bool {
			if !strings.HasSuffix(req.Host, config.Domain) {
				return false
			}
			return strings.TrimSuffix(req.Host, config.Domain) != ""
		}).Handler(http.HandlerFunc(f.ForwardHandler))
	}

	r.HandleFunc("/{bindingId}", f.ForwardHandler)
	port := gautocloud.GetAppInfo().Port
	if port == 0 {
		port = 8089
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func loadLogConfig(c model.Config) {
	if c.LogJson != nil {
		if *c.LogJson {
			log.SetFormatter(&log.JSONFormatter{})
		} else {
			log.SetFormatter(&log.TextFormatter{
				DisableColors: c.LogNoColor,
			})
		}
	}

	if c.LogLevel == "" {
		return
	}
	switch strings.ToUpper(c.LogLevel) {
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
		return
	case "WARN":
		log.SetLevel(log.WarnLevel)
		return
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
		return
	case "PANIC":
		log.SetLevel(log.PanicLevel)
		return
	case "FATAL":
		log.SetLevel(log.FatalLevel)
		return
	}
}
