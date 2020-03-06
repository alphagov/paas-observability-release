package shipper

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	EventsShippedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bosh_auditor_events_shipped_to_splunk_total",
		Help: "Counter of total number of bosh events shipped_to_splunk",
	})
)

func initMetrics() {
	prometheus.MustRegister(EventsShippedTotal)
}
