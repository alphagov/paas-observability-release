package discoverer_test

import (
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"

	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/discoverer"
	fetcherfakes "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher/fakes"
	resolverfakes "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/resolver/fakes"
	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/writer"
)

const (
	project = "my-aiven-project"

	evTimeout  = "5s"
	evInterval = "10ms"

	ctlyTimeout  = "500ms"
	ctlyInterval = "10ms"
)

type fakeWriter struct {
	mostRecentWrite []writer.PrometheusTargetConfig
}

func (w *fakeWriter) Write(targets []writer.PrometheusTargetConfig) {
	w.mostRecentWrite = targets
}

var _ = Describe("Discoverer", func() {
	var (
		d discoverer.Discoverer
		w *fakeWriter
		f *fetcherfakes.FakeFetcher
		r *resolverfakes.FakeResolver

		logger lager.Logger

		discovererDNSDiscoveryErrorsTotal float64
		discovererDNSDiscoveriesTotal     float64
		discovererWriteTargetsTotal       float64
	)

	BeforeEach(func() {
		var err error

		logger = lager.NewLogger("discoverer-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		f = fetcherfakes.NewFakeFetcher()
		r = resolverfakes.NewFakeResolver()
		w = &fakeWriter{}

		d, err = discoverer.NewDiscoverer(
			project,
			f, r, w,
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
	})

	AfterEach(func() {
		By("stopping")
		d.Stop()
	})

	It("should perform a full integration cycle", func() {
		f.ShouldReturn(make([]aiven.Service, 0))
		r.ShouldReturnIPs(make([]net.IP, 0))

		By("starting")
		d.Start()

		By("polling until there are no targets")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(BeEmpty())

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				CloudName: "aws-eu-west-1",
				NodeCount: 3,
				URIParams: map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})
		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4)})

		By("polling until there are targets")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(Equal([]writer.PrometheusTargetConfig{
			{
				Targets: []net.IP{net.IPv4(1, 2, 3, 4)},
				Labels: writer.PrometheusTargetConfigLabels{
					ServiceName: "a-service",
					ServiceType: "elasticsearch",
					Hostname:    "an-instance.aivencloud.com",
					Plan:        "tiny-6.x",
					Cloud:       "aws-eu-west-1",
					NodeCount:   "3",
				},
			},
		}))

		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4), net.IPv4(4, 3, 2, 1)})

		By("polling until the targets have updated")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(Equal([]writer.PrometheusTargetConfig{
			{
				Targets: []net.IP{net.IPv4(1, 2, 3, 4), net.IPv4(4, 3, 2, 1)},
				Labels: writer.PrometheusTargetConfigLabels{
					ServiceName: "a-service",
					ServiceType: "elasticsearch",
					Hostname:    "an-instance.aivencloud.com",
					Plan:        "tiny-6.x",
					Cloud:       "aws-eu-west-1",
					NodeCount:   "3",
				},
			},
		}))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				CloudName: "aws-eu-west-1",
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
				CloudName: "aws-eu-west-1",
				NodeCount: 2,
				URIParams: map[string]string{"host": "another-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})

		By("polling until there are more targets")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(ConsistOf([]writer.PrometheusTargetConfig{
			{
				Targets: []net.IP{
					net.IPv4(1, 2, 3, 4),
					net.IPv4(4, 3, 2, 1),
				},
				Labels: writer.PrometheusTargetConfigLabels{
					ServiceName: "a-service",
					ServiceType: "elasticsearch",
					Hostname:    "an-instance.aivencloud.com",
					Plan:        "tiny-6.x",
					Cloud:       "aws-eu-west-1",
					NodeCount:   "3",
				},
			},
			{
				Targets: []net.IP{
					net.IPv4(1, 2, 3, 4),
					net.IPv4(4, 3, 2, 1),
				},
				Labels: writer.PrometheusTargetConfigLabels{
					ServiceName: "another-service",
					ServiceType: "elasticsearch",
					Hostname:    "another-instance.aivencloud.com",
					Plan:        "tiny-7.x",
					Cloud:       "aws-eu-west-1",
					NodeCount:   "2",
				},
			},
		}))

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:         "a-service-without-prometheus",
				Type:         "influxdb",
				URIParams:    map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{},
			},
		})

		By("polling until there are no targets")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(BeEmpty())
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
	})

	It("should be resilient to errors", func() {
		f.ShouldReturn(make([]aiven.Service, 0))
		r.ShouldReturnError(fmt.Errorf("Can not resolve IPs"))

		By("starting")
		d.Start()

		By("polling until there are no targets")
		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(BeEmpty())

		f.ShouldReturn([]aiven.Service{
			aiven.Service{
				Name:      "a-service",
				Type:      "elasticsearch",
				Plan:      "tiny-6.x",
				CloudName: "aws-eu-west-1",
				NodeCount: 3,
				URIParams: map[string]string{"host": "an-instance.aivencloud.com"},
				Integrations: []*aiven.ServiceIntegration{
					&aiven.ServiceIntegration{IntegrationType: "prometheus"},
				},
			},
		})

		By("polling while there are no targets")
		Consistently(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, ctlyTimeout, ctlyInterval).Should(BeEmpty())

		By("polling until there are targets")
		r.ShouldReturnError(nil)
		r.ShouldReturnIPs([]net.IP{net.IPv4(1, 2, 3, 4)})

		Eventually(func() []writer.PrometheusTargetConfig {
			return w.mostRecentWrite
		}, evTimeout, evInterval).Should(Equal([]writer.PrometheusTargetConfig{
			{
				Targets: []net.IP{net.IPv4(1, 2, 3, 4)},
				Labels: writer.PrometheusTargetConfigLabels{
					ServiceName: "a-service",
					ServiceType: "elasticsearch",
					Hostname:    "an-instance.aivencloud.com",
					Plan:        "tiny-6.x",
					Cloud:       "aws-eu-west-1",
					NodeCount:   "3",
				},
			},
		}))

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
	})
})
