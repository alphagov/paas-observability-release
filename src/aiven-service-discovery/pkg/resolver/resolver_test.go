package resolver_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	r "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/resolver"
	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
)

var _ = Describe("Resolver", func() {
	var (
		resolver r.Resolver

		resolvesTotal        float64
		resolveFailuresTotal float64
	)

	BeforeEach(func() {
		resolver = r.NewResolver()

		By("setting the metric values before each test")
		resolvesTotal = h.CurrentMetricValue(r.ResolverResolvesTotal)
		resolveFailuresTotal = h.CurrentMetricValue(r.ResolverResolveFailuresTotal)
	})

	Context("when resolving succeeds", func() {
		It("should return the IPs and increment the total metric", func() {
			ips, err := resolver.Resolve("127.0.0.1")

			By("checking the results")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(ContainElement(net.IPv4(127, 0, 0, 1)))

			By("checking the metrics")
			Expect(r.ResolverResolvesTotal).To(h.MetricIncrementedBy(resolvesTotal, "==", 1))
			Expect(r.ResolverResolveFailuresTotal).To(h.MetricIncrementedBy(resolveFailuresTotal, "==", 0))
		})
	})

	Context("when resolving fails", func() {
		It("should return an error IPs and increment failure and total metrics", func() {
			ips, err := resolver.Resolve("")

			By("checking the results")
			Expect(err).To(HaveOccurred())
			Expect(ips).To(HaveLen(0))

			By("checking the metrics")
			Expect(r.ResolverResolvesTotal).To(h.MetricIncrementedBy(resolvesTotal, "==", 1))
			Expect(r.ResolverResolveFailuresTotal).To(h.MetricIncrementedBy(resolveFailuresTotal, "==", 1))
		})
	})
})
