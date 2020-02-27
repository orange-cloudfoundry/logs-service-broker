package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/gautocloud"
	_ "github.com/cloudfoundry-community/gautocloud/connectors/databases/gorm"
	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/o1egl/gormrus"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/syslog"
	"github.com/orange-cloudfoundry/logs-service-broker/userdocs"
	"github.com/pivotal-cf/brokerapi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gormigrate.v1"
)

func init() {
	if gautocloud.IsInACloudEnv() && gautocloud.CurrentCloudEnv().Name() != "localcloud" {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func main() {
	panic(boot())
}

func retrieveGormDb(config model.Config) *gorm.DB {
	var db *gorm.DB
	err := gautocloud.Inject(&db)
	if err == nil {
		if config.SQLCnxMaxOpen != 0 {
			db.DB().SetMaxOpenConns(config.SQLCnxMaxOpen)
		}
		if config.SQLCnxMaxIdle != 0 {
			db.DB().SetMaxIdleConns(config.SQLCnxMaxIdle)
		}
		if config.SQLCnxMaxLife != "" {
			duration, err := time.ParseDuration(config.SQLCnxMaxLife)
			if err != nil {
				log.Warnf("Invalid configuration for SQLCnxMaxLife : %s", err.Error())
			} else {
				db.DB().SetConnMaxLifetime(duration)
			}
		}
		return db
	}
	if !config.FallbackToSqlite {
		log.Fatalf("Error when loading database: %s", err.Error())
	}
	log.Warnf("Error when loading database, switching to sqlite, see message: %s", err.Error())
	db, _ = gorm.Open("sqlite3", config.SQLitePath)
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

	port := config.Port
	if gautocloud.GetAppInfo().Port > 0 {
		port = gautocloud.GetAppInfo().Port
	}
	if port == 0 {
		port = 8088
	}
	config.Port = port

	db := retrieveGormDb(config)
	defer db.Close()

	db.SetLogger(gormrus.New())
	if log.GetLevel() == log.DebugLevel {
		db.LogMode(true)
	}
	migrations := Migrations{Config: config, Migrations: gormMigration}
	migrate := gormigrate.New(db, gormigrate.DefaultOptions, migrations.ToGormMigrate())
	migrate.InitSchema(func(db *gorm.DB) error {
		return db.AutoMigrate(
			&model.LogMetadata{},
			&model.InstanceParam{},
			&model.Patterns{},
			&model.Label{},
			&model.SourceLabel{},
		).Error
	})
	if err := migrate.Migrate(); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	migratePatternIfNeeded(db, config.SyslogAddresses)

	sw, err := CreateWriters(config.SyslogAddresses)
	if err != nil {
		return err
	}
	defer func() {
		for _, w := range sw {
			w.Close()
		}
	}()
	c, err := NewMetaCacher(db, config.CacheDuration)
	if err != nil {
		log.Fatal(err)
	}
	go c.Cleaner()

	urlSyslog, _ := url.Parse(config.SyslogDrainURL)
	allowedDomain := urlSyslog.Host
	if urlSyslog.Host == "" {
		allowedDomain = config.SyslogDrainURL
	}
	allowedDomain = strings.Split(allowedDomain, ":")[0]

	forwarder := NewForwarder(c, sw, config.ParsingKeys, allowedDomain, config.DisallowLogsFromExternal)
	broker := NewLoghostBroker(db, c, config)
	userdoc := userdocs.NewUserDoc(db, config)

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

	boxAsset := packr.New("userdocs_assets", "./userdocs/assets")
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(boxAsset)))

	r.Handle("/docs", userdoc)
	r.Handle("/docs/{instanceId}", userdoc)

	if config.VirtualHost {
		url, err := url.Parse(config.SyslogDrainURL)
		if err != nil {
			log.Errorf("unable to parse provided syslog_drain_url '%s' : %s", config.SyslogDrainURL)
			return err
		}
		r.NewRoute().MatcherFunc(func(req *http.Request, m *mux.RouteMatch) bool {
			if !strings.HasSuffix(req.Host, url.Hostname()) {
				return false
			}
			return strings.TrimSuffix(req.Host, url.Hostname()) != ""
		}).Handler(forwarder)
	}

	r.Handle("/{bindingId}", forwarder)

	if !config.NotExitWhenConnFailed {
		go checkDbConnection(db)
	}

	if config.HasTLS() {
		log.Infof("serving https on :%d", config.TLSPort)
		go func() {
			err := http.ListenAndServeTLS(fmt.Sprintf(":%d", config.TLSPort), config.SSLCertFile, config.SSLKeyFile, r)
			if err != nil {
				panic(err)
				os.Exit(1)
			}
		}()

	}
	log.Infof("serving http on :%d", config.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.Port), r)
}

func checkDbConnection(db *gorm.DB) {
	for {
		err := db.DB().Ping()
		if err != nil {
			db.Close()
			log.Fatalf("Error when pinging database: %s", err.Error())
		}
		time.Sleep(5 * time.Minute)
	}
}

func loadLogConfig(c model.Config) {
	if c.LogJSON != nil {
		if *c.LogJSON {
			log.SetFormatter(&log.JSONFormatter{})
		} else {
			log.SetFormatter(&log.TextFormatter{
				DisableColors: c.LogNoColor,
			})
		}
	}
	lvl, err := log.ParseLevel(c.LogLevel)
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(lvl)
	}
}
