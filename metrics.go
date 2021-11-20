package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type metrics struct {
	transferRun prometheus.Counter
	txAmount             prometheus.Counter
	addressTotalBalance  *prometheus.GaugeVec
	addressLockedBalance *prometheus.GaugeVec
}

func initPrometheus(env envConfig, mux *http.ServeMux) *metrics {
	m := &metrics{}

	m.transferRun = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "transfer_runs_count",
		Help:      "Number of transfer runs",
		Namespace: env.MetricsNamespace,
		Subsystem: env.MetricsSubsystem,
	})

	m.txAmount = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "transfer_amount_total",
		Help:      "Amount transferred",
		Namespace: env.MetricsNamespace,
		Subsystem: env.MetricsSubsystem,
	})

	m.addressTotalBalance = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "total_balance",
		Help:      "Total balance of the address",
		Namespace: env.MetricsNamespace,
		Subsystem: env.MetricsSubsystem,
	}, []string{"address"})

	m.addressLockedBalance = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "locked_balance",
		Help:      "Locked balance of the address",
		Namespace: env.MetricsNamespace,
		Subsystem: env.MetricsSubsystem,
	}, []string{"address"})

	mux.Handle(env.MetricsPath, promhttp.Handler())
	return m
}
