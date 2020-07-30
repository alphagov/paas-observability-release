package writer_test

import (
	"io/ioutil"
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"

	h "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/testhelpers"
	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/writer"
)

const (
	evTimeout  = "5s"
	evInterval = "10ms"

	ctlyTimeout  = "500ms"
	ctlyInterval = "10ms"
)

var _ = Describe("Writer", func() {
	var (
		target string
		w      writer.Writer

		logger lager.Logger

		writerWriteTargetsTotal       float64
		writerWriteTargetsErrorsTotal float64
	)

	BeforeEach(func() {
		var err error

		targetFile, err := ioutil.TempFile("", "targets")
		Expect(err).NotTo(HaveOccurred())
		target = targetFile.Name()
		err = targetFile.Close()
		Expect(err).NotTo(HaveOccurred())

		logger = lager.NewLogger("writer-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		w = writer.NewWriter(target, logger)

		By("setting the metric values before each test")
		writerWriteTargetsTotal = h.CurrentMetricValue(
			writer.WriterWriteTargetsTotal,
		)
		writerWriteTargetsErrorsTotal = h.CurrentMetricValue(
			writer.WriterWriteTargetsErrorsTotal,
		)
	})

	AfterEach(func() {
		if target != "" {
			os.Remove(target)
		}
	})

	It("should keep the file up to date with targets", func() {
		By("writing the targets")
		w.Write([]writer.PrometheusTargetConfig{
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
		})
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
                "aiven_cloud": "aws-eu-west-1",
                "aiven_node_count": "3"
            }
        }]`))

		By("updating the targets")
		w.Write([]writer.PrometheusTargetConfig{
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
		})
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
                "aiven_cloud": "aws-eu-west-1",
                "aiven_node_count": "3"
            }
        }]`))

		By("updating the targets several times")
		w.Write([]writer.PrometheusTargetConfig{
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
					net.IPv4(3, 9, 6, 4),
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
		})
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
                "aiven_cloud": "aws-eu-west-1",
                "aiven_node_count": "3"
            }
        }, {
            "targets": ["1.2.3.4", "3.9.6.4"],
            "labels": {
                "aiven_service_name": "another-service",
                "aiven_service_type": "elasticsearch",
                "aiven_hostname": "another-instance.aivencloud.com",
                "aiven_plan": "tiny-7.x",
                "aiven_cloud": "aws-eu-west-1",
                "aiven_node_count": "2"
            }
        }]`))

		By("updating even if there are no services")
		w.Write([]writer.PrometheusTargetConfig{})
		Eventually(func() []byte {
			contents, _ := ioutil.ReadFile(target)
			return contents
		}, evTimeout, evInterval).Should(MatchJSON(`[]`))

		By("checking the metrics")
		Expect(writer.WriterWriteTargetsTotal).To(
			h.MetricIncrementedBy(writerWriteTargetsTotal, "==", 4),
		)
		Expect(writer.WriterWriteTargetsErrorsTotal).To(
			h.MetricIncrementedBy(writerWriteTargetsErrorsTotal, "==", 0),
		)
	})
})
