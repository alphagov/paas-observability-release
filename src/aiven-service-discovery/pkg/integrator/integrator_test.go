package integrator_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jarcoal/httpmock"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"

	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher/fakes"
	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/integrator"
	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
)

const (
	project  = "my-aiven-project"
	token    = "my-aiven-api-token"
	endpoint = "my-aiven-prometheus-endpoint-id"

	evTimeout  = "5s"
	evInterval = "10ms"

	ctlyTimeout  = "500ms"
	ctlyInterval = "10ms"
)

var _ = Describe("Integrator", func() {
	var (
		i integrator.Integrator
		f *fakes.FakeFetcher

		logger lager.Logger

		integratorCreateServiceIntegrationErrorsTotal float64
		integratorCreateServiceIntegrationsTotal      float64
	)

	BeforeSuite(func() {
		httpmock.Activate()
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	BeforeEach(func() {
		var err error

		httpmock.Reset()

		logger = lager.NewLogger("integrator-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		f = fakes.NewFakeFetcher()

		i, err = integrator.NewIntegrator(
			project, token, endpoint,
			f,
			logger,
		)
		Expect(err).NotTo(HaveOccurred())

		i.SetInterval(100 * time.Millisecond) // We want fast tests

		By("setting the metric values before each test")
		integratorCreateServiceIntegrationErrorsTotal = h.CurrentMetricValue(
			integrator.IntegratorCreateServiceIntegrationErrorsTotal,
		)
		integratorCreateServiceIntegrationsTotal = h.CurrentMetricValue(
			integrator.IntegratorCreateServiceIntegrationsTotal,
		)
	})

	AfterEach(func() {
		By("stopping")
		i.Stop()
	})

	It("should perform a full integration cycle", func() {
		httpmock.RegisterResponder(
			"POST",
			fmt.Sprintf("https://api.aiven.io/v1/project/%s/integration", project),
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"errors":              []string{},
				"message":             "Completed",
				"service_integration": aiven.ServiceIntegration{},
			}),
		)

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:         "a-service",
				Type:         "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{},
			},
		})

		By("starting")
		i.Start()

		By("polling for it to create the service integration")
		Eventually(httpmock.GetTotalCallCount).Should(Equal(1))

		By("polling for it to do nothing")
		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name: "a-service",
				Type: "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})
		Consistently(httpmock.GetTotalCallCount, ctlyTimeout, ctlyInterval).Should(Equal(1))

		By("polling for it to create another service integration")
		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name: "a-service",
				Type: "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
			aiven.Service{
				Name:         "another-service",
				Type:         "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{},
			},
		})
		Eventually(httpmock.GetTotalCallCount, evTimeout, evInterval).Should(Equal(2))

		By("polling for it to do nothing again")
		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name: "a-service",
				Type: "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
			aiven.Service{
				Name: "another-service",
				Type: "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})
		Consistently(httpmock.GetTotalCallCount, ctlyTimeout, ctlyInterval).Should(Equal(2))

		By("checking the metrics")
		Expect(integrator.IntegratorCreateServiceIntegrationsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationsTotal, ">=", 2),
		)
		Expect(integrator.IntegratorCreateServiceIntegrationErrorsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationErrorsTotal, "==", 0),
		)
	})

	It("should be resilient to errors", func() {
		creations := 0

		httpmock.RegisterResponder(
			"POST",
			fmt.Sprintf("https://api.aiven.io/v1/project/%s/integration", project),
			func(req *http.Request) (*http.Response, error) {
				var resp *http.Response

				switch httpmock.GetTotalCallCount() {
				case 5:
					resp, _ = httpmock.NewJsonResponse(200, map[string]interface{}{
						"errors":              []string{},
						"message":             "Completed",
						"service_integration": aiven.ServiceIntegration{},
					})
					creations++
				default:
					resp = httpmock.NewStringResponse(404, "")
				}

				return resp, nil
			},
		)

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:         "a-service",
				Type:         "elasticsearch",
				Integrations: []*aiven.ServiceIntegration{},
			},
		})

		By("starting")
		i.Start()

		By("polling for it to create the service integration after some errors")
		Eventually(func() int { return creations }, evTimeout, evInterval).Should(Equal(1))
		Expect(httpmock.GetTotalCallCount()).To(BeNumerically(">=", 5))

		By("checking the metrics")
		Expect(integrator.IntegratorCreateServiceIntegrationsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationsTotal, ">=", 1),
		)
		Expect(integrator.IntegratorCreateServiceIntegrationErrorsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationErrorsTotal, ">=", 1),
		)
	})

	It("should not create service integrations for ineligible services", func() {
		httpmock.RegisterResponder(
			"POST",
			fmt.Sprintf("https://api.aiven.io/v1/project/%s/integration", project),
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"errors":              []string{},
				"message":             "Completed",
				"service_integration": aiven.ServiceIntegration{},
			}),
		)

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:         "a-service",
				Type:         "influxdb",
				Integrations: []*aiven.ServiceIntegration{},
			},
		})

		By("starting")
		i.Start()

		By("polling for it to do nothing for ineligible services")
		Consistently(httpmock.GetTotalCallCount, ctlyTimeout, ctlyInterval).Should(Equal(0))

		By("checking the metrics")
		Expect(integrator.IntegratorCreateServiceIntegrationsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationsTotal, "==", 0),
		)
		Expect(integrator.IntegratorCreateServiceIntegrationErrorsTotal).To(
			h.MetricIncrementedBy(integratorCreateServiceIntegrationErrorsTotal, "==", 0),
		)
	})
})
