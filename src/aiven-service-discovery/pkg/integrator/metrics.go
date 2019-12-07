package integrator

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	IntegratorCreateServiceIntegrationErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "integrator_create_service_integration_errors_total",
		Help: "Counter of total number of Aiven create service integration failures",
	})

	IntegratorCreateServiceIntegrationsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "integrator_create_service_integrations_total",
		Help: "Counter of total number of calls to create a service integration",
	})
)

func initMetrics() {
	prometheus.MustRegister(IntegratorCreateServiceIntegrationErrorsTotal)
	prometheus.MustRegister(IntegratorCreateServiceIntegrationsTotal)
}
