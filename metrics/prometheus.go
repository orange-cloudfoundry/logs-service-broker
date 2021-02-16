package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	LogsSentFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_errors_total",
			Help: "Number of non transmitted logs due to failures.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	LogsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_total",
			Help: "Number of transmitted logs.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	LogsSentWithoutCache = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_without_cache_total",
			Help: "Number of transmitted logs without cache system.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	LogsSentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "logs_sent_duration",
			Help:    "Summary of logs sent duration.",
			Buckets: []float64{0.005, 0.01, 0.1, 0.25, 0.5, 1},
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	DbStatsCnxMaxOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_max_open",
			Help: "Maximum number of open connections to the database",
		},
	)
	DbStatsCnxUsed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_used",
			Help: "The number of connections currently in use",
		},
	)
	DbStatsCnxIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_idle",
			Help: "The number of idle connections",
		},
	)
	DbStatsCnxWaitDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_wait_duration",
			Help: "The total time blocked waiting for a new connection in seconds",
		},
	)
	DbStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_status",
			Help: "Current status of database connectivity, 0 is error",
		},
	)
)

func init() {
	prometheus.MustRegister(LogsSentFailure)
	prometheus.MustRegister(LogsSent)
	prometheus.MustRegister(LogsSentDuration)
	prometheus.MustRegister(LogsSentWithoutCache)
	prometheus.MustRegister(DbStatsCnxMaxOpen)
	prometheus.MustRegister(DbStatsCnxUsed)
	prometheus.MustRegister(DbStatsCnxIdle)
	prometheus.MustRegister(DbStatsCnxWaitDuration)
	prometheus.MustRegister(DbStatus)
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
