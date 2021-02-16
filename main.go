package main

import (
	"context"
	"fmt"
	"github.com/orange-cloudfoundry/logs-service-broker/api"
	"github.com/orange-cloudfoundry/logs-service-broker/dbservices"
	"github.com/orange-cloudfoundry/logs-service-broker/metrics"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/gormigrate.v1"
)

type writerMap = map[string]io.WriteCloser
type app struct {
	config *model.Config
}

func main() {
	kingpin.Version(version.Print("logs-service-broker"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	a := newApp()
	a.run()
}

func newApp() *app {
	var config model.Config
	if err := gautocloud.Inject(&config); err != nil {
		log.Fatalf("unable to load configuration: %s", err)
	}

	return &app{
		config: &config,
	}
}

// run -
// 1. important to register as last handler cause it will capture `/(.*)` paths
func (a *app) run() {
	a.initializeLogs()

	db, err := a.initializeDB()
	if err != nil {
		log.Fatalf("unable to loading database: %s", err.Error())
	}

	err = a.migrateDB(db)
	if err != nil {
		log.Fatalf("unable to migrate database: %v", err.Error())
	}

	cacher, err := a.initializeMetaCache(db)
	if err != nil {
		log.Fatalf("unable to initialize meta cacher: %s", err.Error())
	}

	writers, err := a.initializeWriters()
	if err != nil {
		log.Fatalf("unable to create syslog writers: %s", err.Error())
	}

	router := mux.NewRouter()
	a.registerBroker(router, db, cacher)
	a.registerDoc(router, db)
	a.registerMetrics(router)
	a.registerProfiler(router)
	// 1.
	a.registerForwarder(router, writers, cacher)

	a.listen(router)
	a.finish(db, writers)
}

func (a *app) finish(db *gorm.DB, writers writerMap) {
	if db != nil {
		db.Close()
	}
	if writers != nil {
		for _, w := range writers {
			w.Close()
		}
	}
}

func (a *app) initializeLogs() {
	if a.config.Log.JSON != nil {
		if *a.config.Log.JSON {
			log.SetFormatter(&log.JSONFormatter{})
		} else {
			log.SetFormatter(&log.TextFormatter{
				DisableColors: a.config.Log.NoColor,
			})
		}
	}
	lvl, err := log.ParseLevel(a.config.Log.Level)
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(lvl)
	}
}

// reconnectDB -
// inspired by gorm author plugin: https://git.feneas.org/ganggo/gorm/-/blob/615ff81ac106969ebe511a34f9770085f73a57f3/plugins/reconnect/reconnect.go
func (a *app) reconnectDB(db *gorm.DB) error {
	var newDB *gorm.DB

	err := gautocloud.Inject(&newDB)
	if err != nil {
		log.Errorf("[watchdog] failed to re-create database: %s", err.Error())
		return err
	}

	(*db.DB()) = *(newDB.DB())
	err = db.DB().Ping()
	if err != nil {
		log.Errorf("[watchdog] failed to ping database after recreate: %s", err.Error())
		return err
	}
	return nil
}

// initializeDB -
// 1. create DB according to config, fallback to sqlite if applicable
// 2. configure connexion pools
// 3. configure logs
// 3. run background db watcher if needed
func (a *app) initializeDB() (*gorm.DB, error) {
	var db *gorm.DB

	// 1.
	err := gautocloud.Inject(&db)
	if err != nil && a.config.DB.SQLiteFallback {
		log.Warnf("Error when loading database, switching to sqlite, see message: %s", err.Error())
		db, err = gorm.Open("sqlite3", a.config.DB.SQLitePath)
	}
	if err != nil {
		return nil, err
	}

	// 2.
	if a.config.DB.CnxMaxOpen != 0 {
		db.DB().SetMaxOpenConns(a.config.DB.CnxMaxOpen)
	}
	if a.config.DB.CnxMaxIdle != 0 {
		db.DB().SetMaxIdleConns(a.config.DB.CnxMaxIdle)
	}
	if a.config.DB.CnxMaxLife != "" {
		duration, err := time.ParseDuration(a.config.DB.CnxMaxLife)
		if err != nil {
			log.Warnf("Invalid configuration for SQLCnxMaxLife : %s", err.Error())
		} else {
			db.DB().SetConnMaxLifetime(duration)
		}
	}

	// 3.
	db.SetLogger(gormrus.New())
	if log.GetLevel() == log.DebugLevel {
		db.LogMode(true)
	}

	// 4.
	a.watchDB(db)
	return db, nil
}

// watchDB - background thread that exit program if DB is not reachable
func (a *app) watchDB(db *gorm.DB) {
	go func() {
		for {
			stats := db.DB().Stats()
			metrics.DbStatsCnxMaxOpen.Set(float64(stats.MaxOpenConnections))
			metrics.DbStatsCnxUsed.Set(float64(stats.InUse))
			metrics.DbStatsCnxIdle.Set(float64(stats.Idle))
			metrics.DbStatsCnxWaitDuration.Set(stats.WaitDuration.Seconds())

			err := db.DB().Ping()
			if err != nil {
				log.Errorf("[watchdog] failed to ping database, try reconnect: %s", err.Error())
				if err = a.reconnectDB(db); err != nil {
					metrics.DbStatus.Set(float64(0))
					time.Sleep(30 * time.Second)
					continue
				}
			}
			metrics.DbStatus.Set(float64(1))
			time.Sleep(2 * time.Minute)
		}
	}()
}

func (a *app) migrateDB(db *gorm.DB) error {
	migrations := dbservices.Migrations{
		Config:     a.config,
		Migrations: dbservices.GormMigration(),
	}
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
	return migrate.Migrate()
}

// listen -
// 1. listen for SIGUSR1 or Interrupt signals
// 2. create servers and serve requests
// 3. wait for signal trigger
// 4. graceful shutdown servers with timeout
func (a *app) listen(h http.Handler) {
	// 1.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		oscall := <-c
		log.Infof("received signal: %+v", oscall)
		cancel()
	}()

	// 2.
	var server, serverTLS *http.Server

	server = a.startServer(h, a.config.Web.GetPort(), nil, nil)
	if a.config.HasTLS() {
		serverTLS = a.startServer(h, a.config.Web.TLS.Port, &a.config.Web.TLS.CertFile, &a.config.Web.TLS.KeyFile)
	}

	// 3.
	<-ctx.Done()

	// 4.
	log.Infof("server shutdown required")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
	}()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server Shutdown Failed:%+s", err)
	}
	if serverTLS != nil {
		if err := serverTLS.Shutdown(ctx); err != nil {
			log.Fatalf("server Shutdown Failed:%+s", err)
		}
	}
	log.Infof("server shutdown complete")
}

func (a *app) startServer(h http.Handler, port int, certFile, keyFile *string) *http.Server {
	rand.Seed(time.Now().UnixNano())

	cnxCtxFn := func(ctx context.Context, c net.Conn) context.Context {
		return ctx
	}

	if !a.config.Web.MaxKeepAlive.Disabled {
		seconds := a.config.Web.MaxKeepAlive.GetFuzziness().Seconds()
		cnxCtxFn = func(ctx context.Context, c net.Conn) context.Context {
			fuzziness := time.Duration(seconds * rand.Float64())
			end := time.
				Now().
				Add(*a.config.Web.MaxKeepAlive.GetDuration()).
				Add(fuzziness)
			return context.WithValue(ctx, model.EndOfLifeKey, end)
		}
	}

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:        addr,
		Handler:     h,
		ConnContext: cnxCtxFn,
	}
	go func() {
		var err error
		if certFile != nil && keyFile != nil {
			log.Infof("serving https on %s", addr)
			err = server.ListenAndServeTLS(*certFile, *keyFile)
		} else {
			log.Infof("serving http on %s", addr)
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			panic(err)
			os.Exit(1)
		}
	}()
	return server
}

// maxKeepAliveDecorator -
// Enforce "Connection: close" header for connexions existing for more then
// maxKeepAlive. Forcing client to reconnect and possibly to rebalance to other
// nodes.
// 1. disabled when maxKeepAlive is zero
func (a *app) maxKeepAliveDecorator(h http.Handler) http.Handler {
	// 1.
	if a.config.Web.MaxKeepAlive.Disabled {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endTimeI := r.Context().Value(model.EndOfLifeKey)
		endTime := endTimeI.(time.Time)
		if time.Now().After(endTime) {
			w.Header().Set("Connection", "close")
		}
		h.ServeHTTP(w, r)
	})
}

func (a *app) initializeMetaCache(db *gorm.DB) (*dbservices.MetaCacher, error) {
	cacher, err := dbservices.NewMetaCacher(db, a.config.BindingCache.Duration)
	if err != nil {
		return nil, err
	}
	go cacher.Cleaner()
	if a.config.BindingCache.PreCache {
		err := cacher.PreCache()
		if err != nil {
			return nil, err
		}
	}

	return cacher, nil
}

func (a *app) initializeWriters() (writerMap, error) {
	writers := make(writerMap)
	for _, sysAddr := range a.config.SyslogAddresses {
		writer, err := syslog.NewWriter(sysAddr.URLs...)
		if err != nil {
			return nil, err
		}
		writers[sysAddr.Name] = writer
	}
	return writers, nil
}

// registerBroker
// 1. create broker implementation instance
// 2. decorate with standard web broker interface
// 3. bind to /v2 routes
func (a *app) registerBroker(router *mux.Router, db *gorm.DB, cacher *dbservices.MetaCacher) {
	// 1.
	broker := api.NewLoghostBroker(db, cacher, a.config)

	// 2.
	lag := lager.NewLogger("broker")
	lag.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerHandler := brokerapi.New(broker, lag, brokerapi.BrokerCredentials{
		Username: a.config.Broker.Username,
		Password: a.config.Broker.Password,
	})

	// 3.
	matcherFunc := func(req *http.Request, m *mux.RouteMatch) bool {
		return strings.HasPrefix(req.URL.Path, "/v2")
	}
	router.NewRoute().MatcherFunc(matcherFunc).Handler(brokerHandler)
}

func (a *app) registerDoc(router *mux.Router, db *gorm.DB) {
	userdoc := userdocs.NewUserDoc(db, a.config)
	assets := packr.New("userdocs_assets", "./userdocs/assets")
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(assets)))
	router.Handle("/docs", userdoc)
	router.Handle("/docs/{instanceId}", userdoc)
}

// registerForwarder
// 1. wrap forward handler with auto-close cnx decorator
// 2. handle request like '{bindingID}.{drainHost}'
func (a *app) registerForwarder(router *mux.Router, writers writerMap, cacher *dbservices.MetaCacher) {
	f := api.NewForwarder(cacher, writers, a.config)

	decorated := a.maxKeepAliveDecorator(f)

	router.Handle("/{bindingId}", decorated)
}

func (a *app) registerMetrics(router *mux.Router) {
	router.Handle("/metrics", promhttp.Handler())
}

func (a *app) registerProfiler(router *mux.Router) {
	if !a.config.Log.EnableProfiler {
		return
	}
	log.Warn("enabling profiling endpoint")
	router.HandleFunc("/debug/pprof", pprof.Index)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func init() {
	if gautocloud.IsInACloudEnv() && gautocloud.CurrentCloudEnv().Name() != "localcloud" {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
