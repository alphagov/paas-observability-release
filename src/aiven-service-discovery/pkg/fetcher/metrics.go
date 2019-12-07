package fetcher

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	FetcherAivenListServicesErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fetcher_aiven_service_list_errors_total",
		Help: "Counter of total number of Aiven list services API failures",
	})

	FetcherFetchesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fetcher_fetches_total",
		Help: "Counter of total number of fetcher calls",
	})
)

func initMetrics() {
	prometheus.MustRegister(FetcherAivenListServicesErrorsTotal)
	prometheus.MustRegister(FetcherFetchesTotal)
}
