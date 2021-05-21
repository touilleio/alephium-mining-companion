package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type metrics struct {
	transferRun prometheus.Counter
}

func initPrometheus(env envConfig, mux *http.ServeMux) *metrics {
	m := &metrics{}

	m.transferRun = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "transfer_runs_count",
		Help:      "Number of transfer runs",
		Namespace: env.MetricsNamespace,
		Subsystem: env.MetricsSubsystem,
	})

	mux.Handle(env.MetricsPath, promhttp.Handler())
	return m
}
