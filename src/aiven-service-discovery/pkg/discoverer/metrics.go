package discoverer

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	DiscovererWriteTargetsErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discoverer_write_targets_errors_total",
		Help: "Counter of total number of target file write failures",
	})

	DiscovererWriteTargetsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discoverer_write_targets_total",
		Help: "Counter of total number of target file writes",
	})

	DiscovererDNSDiscoveryErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discoverer_dns_discovery_errors_total",
		Help: "Counter of total number of DNS discovery errors",
	})

	DiscovererDNSDiscoveriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discoverer_dns_discoveries_total",
		Help: "Counter of total number of DNS discoveries",
	})
)

func initMetrics() {
	prometheus.MustRegister(DiscovererWriteTargetsErrorsTotal)
	prometheus.MustRegister(DiscovererWriteTargetsTotal)

	prometheus.MustRegister(DiscovererDNSDiscoveryErrorsTotal)
	prometheus.MustRegister(DiscovererDNSDiscoveriesTotal)
}
