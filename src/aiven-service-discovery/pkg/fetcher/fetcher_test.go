package fetcher_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jarcoal/httpmock"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher"
	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
)

const (
	project = "my-aiven-project"
	token   = "my-aiven-api-token"

	evTimeout  = "5s"
	evInterval = "10ms"
)

var _ = Describe("Fetcher", func() {
	var (
		f        fetcher.Fetcher
		logger   lager.Logger
		registry *prometheus.Registry

		fetchAivenListServicesErrorsTotal float64
		fetchesTotal                      float64
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

		logger = lager.NewLogger("fetcher-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		f, err = fetcher.NewFetcher(project, token, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		By("checking before starting")
		Expect(f.Services()).To(HaveLen(0))

		By("setting the metric values before each test")
		fetchesTotal = h.CurrentMetricValue(
			fetcher.FetcherFetchesTotal,
		)
		fetchAivenListServicesErrorsTotal = h.CurrentMetricValue(
			fetcher.FetcherAivenListServicesErrorsTotal,
		)

		f.SetInterval(100 * time.Millisecond) // We want fast tests

		By("starting")
		f.Start()
	})

	AfterEach(func() {
		By("stopping")
		f.Stop()
	})

	It("should perform a full fetch cycle", func() {
		calls := 0

		httpmock.RegisterResponder(
			"GET",
			fmt.Sprintf("https://api.aiven.io/v1/project/%s/service", project),
			func(req *http.Request) (*http.Response, error) {
				calls++

				var services []aiven.Service
				switch calls {
				case 1:
					services = []aiven.Service{
						aiven.Service{Name: "a-service"},
					}
				case 2:
					services = []aiven.Service{
						aiven.Service{Name: "a-service"},
						aiven.Service{Name: "another-service"},
					}
				default:
					services = []aiven.Service{}
				}

				resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
					"errors":   []string{},
					"message":  "Completed",
					"services": services,
				})

				return resp, nil
			},
		)

		By("polling")
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(0))
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(1))
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(2))
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(0))

		By("checking the metrics")
		Expect(fetcher.FetcherFetchesTotal).To(
			h.MetricIncrementedBy(fetchesTotal, ">=", 3),
		)
		Expect(fetcher.FetcherAivenListServicesErrorsTotal).To(
			h.MetricIncrementedBy(fetchAivenListServicesErrorsTotal, "==", 0),
		)
	})

	It("should be resilient to errors", func() {
		calls := 0

		httpmock.RegisterResponder(
			"GET",
			fmt.Sprintf("https://api.aiven.io/v1/project/%s/service", project),
			func(req *http.Request) (*http.Response, error) {
				calls++

				var services []aiven.Service
				switch calls {
				case 2:
					return httpmock.NewStringResponse(404, ""), nil
				case 5:
					services = []aiven.Service{
						aiven.Service{Name: "a-service"},
						aiven.Service{Name: "another-service"},
					}
				default:
					services = []aiven.Service{
						aiven.Service{Name: "a-service"},
					}
				}

				resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
					"errors":   []string{},
					"message":  "Completed",
					"services": services,
				})

				return resp, nil
			},
		)

		By("polling")
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(1))
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(2))
		Eventually(f.Services, evTimeout, evInterval).Should(HaveLen(1))

		By("checking the metrics")
		Expect(fetcher.FetcherFetchesTotal).To(
			h.MetricIncrementedBy(fetchesTotal, ">=", 6),
		)
		Expect(fetcher.FetcherAivenListServicesErrorsTotal).To(
			h.MetricIncrementedBy(fetchAivenListServicesErrorsTotal, ">=", 1),
		)
	})
})
