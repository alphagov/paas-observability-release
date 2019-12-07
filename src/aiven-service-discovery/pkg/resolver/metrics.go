package resolver

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ResolverResolveFailuresTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "resolver_resolve_failures_total",
		Help: "Counter of total IP resolver calls which returned errors ",
	})

	ResolverResolvesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "resolver_resolves_total",
		Help: "Counter of total number of IP resolver calls",
	})
)

func initMetrics() {
	prometheus.MustRegister(ResolverResolveFailuresTotal)
	prometheus.MustRegister(ResolverResolvesTotal)
}
