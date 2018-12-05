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
		[]string{"instance_id", "binding_id", "plan_name", "org_id", "space_id", "app_id"},
	)
	logsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_sent_total",
			Help: "Number of transmitted logs.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org_id", "space_id", "app_id"},
	)
	logsSentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "logs_sent_duration",
			Help: "Summary of logs sent duration.",
		},
		[]string{"instance_id", "binding_id", "plan_name", "org_id", "space_id", "app_id"},
	)
)

func init() {
	prometheus.MustRegister(logsSentFailure)
	prometheus.MustRegister(logsSent)
	prometheus.MustRegister(logsSentDuration)
}
