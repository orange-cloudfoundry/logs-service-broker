package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	logsSentFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_errors_total",
			Help: "Number of non transmitted logs due to failures.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	logsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_total",
			Help: "Number of transmitted logs.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	logsSentWithoutCache = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_without_cache_total",
			Help: "Number of transmitted logs without cache system.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	logsSentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "logs_sent_duration",
			Help:    "Summary of logs sent duration.",
			Buckets: []float64{0.005, 0.01, 0.1, 0.25, 0.5, 1},
		},
		[]string{"instance_id", "binding_id", "plan_name", "org", "space", "app"},
	)
	dbStatsCnxMaxOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_max_open",
			Help: "Maximum number of open connections to the database",
		},
	)
	dbStatsCnxUsed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_used",
			Help: "The number of connections currently in use",
		},
	)
	dbStatsCnxIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_idle",
			Help: "The number of idle connections",
		},
	)
	dbStatsCnxWaitDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_cnx_wait_duration",
			Help: "The total time blocked waiting for a new connection in seconds",
		},
	)
	dbStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "logs_db_status",
			Help: "Current status of database connectivity, 0 is error",
		},
	)
)

func init() {
	prometheus.MustRegister(logsSentFailure)
	prometheus.MustRegister(logsSent)
	prometheus.MustRegister(logsSentDuration)
	prometheus.MustRegister(logsSentWithoutCache)
	prometheus.MustRegister(dbStatsCnxMaxOpen)
	prometheus.MustRegister(dbStatsCnxUsed)
	prometheus.MustRegister(dbStatsCnxIdle)
	prometheus.MustRegister(dbStatsCnxWaitDuration)
	prometheus.MustRegister(dbStatus)
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
