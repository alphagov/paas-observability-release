package discoverer_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"

	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/discoverer"
	fetcherfakes "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher/fakes"
	resolverfakes "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/resolver/fakes"
	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
)

const (
	project = "my-aiven-project"

	evTimeout  = "5s"
	evInterval = "10ms"

	ctlyTimeout  = "500ms"
	ctlyInterval = "10ms"
)

var _ = Describe("Discoverer", func() {
	var (
		d discoverer.Discoverer
		f *fetcherfakes.FakeFetcher
		r *resolverfakes.FakeResolver

		target string

		logger lager.Logger

		discovererDNSDiscoveryErrorsTotal float64
		discovererDNSDiscoveriesTotal     float64
		discovererWriteTargetsErrorsTotal float64
		discovererWriteTargetsTotal       float64
	)

	BeforeEach(func() {
		var err error

		targetFile, err := ioutil.TempFile("", "targets")
		Expect(err).NotTo(HaveOccurred())
		target = targetFile.Name()
		err = targetFile.Close()
		Expect(err).NotTo(HaveOccurred())

		logger = lager.NewLogger("discoverer-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		f = fetcherfakes.NewFakeFetcher()
		r = resolverfakes.NewFakeResolver()

		d, err = discoverer.NewDiscoverer(
			project, target,
			f, r,
			logger,
		)
		Expect(err).NotTo(HaveOccurred())

		d.SetInterval(100 * time.Millisecond) // We want fast tests

		By("setting the metric values before each test")
		discovererDNSDiscoveriesTotal = h.CurrentMetricValue(
			discoverer.DiscovererDNSDiscoveriesTotal,
		)
		discovererDNSDiscoveryErrorsTotal = h.CurrentMetricValue(
			discoverer.DiscovererDNSDiscoveryErrorsTotal,
		)
		discovererWriteTargetsTotal = h.CurrentMetricValue(
			discoverer.DiscovererWriteTargetsTotal,
		)
		discovererWriteTargetsErrorsTotal = h.CurrentMetricValue(
			discoverer.DiscovererWriteTargetsErrorsTotal,
		)
	})

	AfterEach(func() {
		By("stopping")
		d.Stop()

		if target != "" {
			os.Remove(target)
		}
	})

	It("should perform a full integration cycle", func() {
		f.ShouldReturn(make([]aiven.Service, 0))
		r.ShouldReturnIPs(make([]net.IP, 0))

		By("starting")
		d.Start()

		By("polling until there are no targets")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[]`))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				NodeCount: 3,
				URIParams: map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})
		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4)})

		By("polling until there are targets")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[{
			"targets": ["1.2.3.4"],
			"labels": {
				"aiven_service_name": "a-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "an-instance.aivencloud.com",
				"aiven_plan": "tiny-6.x",
				"aiven_node_count": "3"
			}
		}]`))

		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4), net.IPv4(4, 3, 2, 1)})

		By("polling until the targets have updated")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[{
			"targets": ["1.2.3.4", "4.3.2.1"],
			"labels": {
				"aiven_service_name": "a-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "an-instance.aivencloud.com",
				"aiven_plan": "tiny-6.x",
				"aiven_node_count": "3"
			}
		}]`))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				NodeCount: 3,
				URIParams: map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
			aiven.Service{
				Name:      "another-service",
				Type:      "elasticsearch",
				Plan:      "tiny-7.x",
				NodeCount: 2,
				URIParams: map[string]string{"host": "another-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})

		By("polling until there are more targets")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(Or(MatchJSON(`[{
			"targets": ["1.2.3.4", "4.3.2.1"],
			"labels": {
				"aiven_service_name": "a-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "an-instance.aivencloud.com",
				"aiven_plan": "tiny-6.x",
				"aiven_node_count": "3"
			}
		}, {
			"targets": ["1.2.3.4", "4.3.2.1"],
			"labels": {
				"aiven_service_name": "another-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "another-instance.aivencloud.com",
				"aiven_plan": "tiny-7.x",
				"aiven_node_count": "2"
			}
		}]`), MatchJSON(`[{
			"targets": ["1.2.3.4", "4.3.2.1"],
			"labels": {
				"aiven_service_name": "another-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "another-instance.aivencloud.com",
				"aiven_plan": "tiny-7.x",
				"aiven_node_count": "2"
			}
		}, {
			"targets": ["1.2.3.4", "4.3.2.1"],
			"labels": {
				"aiven_service_name": "a-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "an-instance.aivencloud.com",
				"aiven_plan": "tiny-6.x",
				"aiven_node_count": "3"
			}
		}]`)))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:         "a-service-without-prometheus",
				Type:         "influxdb",
				URIParams:    map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{},
			},
		})

		By("polling until there are no targets")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[]`))

		By("checking the metrics")
		Expect(discoverer.DiscovererDNSDiscoveriesTotal).To(
			h.MetricIncrementedBy(discovererDNSDiscoveriesTotal, ">=", 3),
		)
		Expect(discoverer.DiscovererDNSDiscoveryErrorsTotal).To(
			h.MetricIncrementedBy(discovererDNSDiscoveryErrorsTotal, "==", 0),
		)
		Expect(discoverer.DiscovererWriteTargetsTotal).To(
			h.MetricIncrementedBy(discovererWriteTargetsTotal, ">=", 1),
		)
		Expect(discoverer.DiscovererWriteTargetsErrorsTotal).To(
			h.MetricIncrementedBy(discovererWriteTargetsErrorsTotal, "==", 0),
		)
	})

	It("should be resilient to errors", func() {
		f.ShouldReturn(make([]aiven.Service, 0))
		r.ShouldReturnError(fmt.Errorf("Can not resolve IPs"))

		By("starting")
		d.Start()

		By("polling until there are no targets")
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[]`))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				NodeCount: 3,
				URIParams: map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})

		By("polling while there are no targets")
		Consistently(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, ctlyTimeout, ctlyInterval).Should(MatchJSON(`[]`))

		By("polling until there are targets")
		r.ShouldReturnError(nil)
		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4)})

		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[{
			"targets": ["1.2.3.4"],
			"labels": {
				"aiven_service_name": "a-service",
				"aiven_service_type": "elasticsearch",
				"aiven_hostname": "an-instance.aivencloud.com",
				"aiven_plan": "tiny-6.x",
				"aiven_node_count": "3"
			}
		}]`))

		By("checking the metrics")
		Expect(discoverer.DiscovererDNSDiscoveriesTotal).To(
			h.MetricIncrementedBy(discovererDNSDiscoveriesTotal, ">=", 3),
		)
		Expect(discoverer.DiscovererDNSDiscoveryErrorsTotal).To(
			h.MetricIncrementedBy(discovererDNSDiscoveryErrorsTotal, ">=", 1),
		)
		Expect(discoverer.DiscovererWriteTargetsTotal).To(
			h.MetricIncrementedBy(discovererWriteTargetsTotal, ">=", 1),
		)
		Expect(discoverer.DiscovererWriteTargetsErrorsTotal).To(
			h.MetricIncrementedBy(discovererWriteTargetsErrorsTotal, "==", 0),
		)
	})
})
