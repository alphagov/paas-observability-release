package shipper_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/jarcoal/httpmock"

	c "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/cursor"
	f "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/fetcher"
	s "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/shipper"
)

const (
	splunkURL = "http://splunk.api/hec-endpoint"
)

var _ = Describe("Shipper", func() {

	BeforeSuite(func() {
		httpmock.Activate()
	})

	BeforeEach(func() {
		httpmock.Reset()
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	var (
		err       error
		cursorDir string

		cursor  c.Cursor
		fetcher f.Fetcher
		shipper s.Shipper
		logger  lager.Logger
	)

	BeforeEach(func() {
		logger = lager.NewLogger("shipper-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		cursorDir = ""
		cursorDir, err = ioutil.TempDir("", "shipper-test")
		Expect(err).NotTo(HaveOccurred())

		cursor = c.NewFileCursor(
			"bosh-auditor-shipper-test",
			cursorDir,
			time.Unix(0, 0),
			logger.Session("file-cursor"),
		)
	})

	AfterEach(func() {
		if cursorDir != "" {
			err = os.RemoveAll(cursorDir)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("appears to work", func() {
		fetcher = func(t time.Time) ([]boshdir.Event, error) {
			return []boshdir.Event{
				boshdir.NewEventFromResp(boshdir.Client{}, boshdir.EventResp{
					ID:             "abcd",
					Timestamp:      1234,
					User:           "some-user",
					Action:         "some-action",
					TaskID:         "some-task",
					DeploymentName: "some-deployment",
					Instance:       "some-instance",
				}),
				boshdir.NewEventFromResp(boshdir.Client{}, boshdir.EventResp{
					ID:             "efgh",
					Timestamp:      1235,
					User:           "some-user",
					Action:         "some-action",
					TaskID:         "some-task",
					DeploymentName: "some-deployment",
					Instance:       "some-instance",
				}),
				boshdir.NewEventFromResp(boshdir.Client{}, boshdir.EventResp{
					ID:             "ijkl",
					Timestamp:      1236,
					User:           "some-user",
					Action:         "some-action",
					TaskID:         "some-task",
					DeploymentName: "some-deployment",
					Instance:       "some-instance",
				}),
			}, nil
		}

		shipper = s.NewShipper(
			10*time.Millisecond,
			logger,
			cursor,
			fetcher,
			"dev", "splunk-key", splunkURL,
		)

		httpmock.RegisterResponder(
			"POST", splunkURL,
			func(req *http.Request) (*http.Response, error) {
				body, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())

				var event s.SplunkEvent
				err = json.Unmarshal(body, &event)
				Expect(err).NotTo(HaveOccurred())

				Expect(event).To(MatchAllFields(Fields{
					"SourceType": Equal("bosh-audit-event"),
					"Source":     Equal("dev"),
					"Event": MatchAllFields(Fields{
						"ID": Or(
							Equal("abcd"),
							Equal("efgh"),
							Equal("ijkl"),
						),

						"Timestamp": Or(
							BeNumerically("==", int64(1234)),
							BeNumerically("==", int64(1235)),
							BeNumerically("==", int64(1236)),
						),

						"User":           Equal("some-user"),
						"Action":         Equal("some-action"),
						"TaskID":         Equal("some-task"),
						"DeploymentName": Equal("some-deployment"),
						"Instance":       Equal("some-instance"),
					}),
				}))

				return httpmock.NewJsonResponse(200, map[string]interface{}{
					"message": "success",
				})
			},
		)

		var (
			shipError error
			shipWG    sync.WaitGroup
		)

		shipContext, cancelShip := context.WithTimeout(
			context.Background(), 100*time.Millisecond,
		)

		By("running the shipper")
		shipWG.Add(1)
		go func() {
			defer GinkgoRecover()
			shipError = shipper.Run(shipContext)
			shipWG.Done()
		}()

		By("waiting for events to be shipped")
		Eventually(
			httpmock.GetTotalCallCount, "1000ms", "1ms",
		).Should(BeNumerically("==", 3))

		Expect(shipError).NotTo(HaveOccurred())

		By("checking the cursor was updated")
		Expect(cursor.GetTime()).To(BeTemporally("~", time.Unix(1236, 0)))

		By("cleaning up")
		cancelShip()
		shipWG.Wait()
		Expect(shipError).NotTo(HaveOccurred())
	})

	It("is resilient to errors", func() {
		fetcherCallCount := 0

		fetcher = func(t time.Time) ([]boshdir.Event, error) {
			fetcherCallCount++

			if fetcherCallCount == 2 {
				return []boshdir.Event{}, fmt.Errorf("random error")
			}

			return []boshdir.Event{
				boshdir.NewEventFromResp(boshdir.Client{}, boshdir.EventResp{
					ID:             "abcd",
					Timestamp:      int64(fetcherCallCount),
					User:           "some-user",
					Action:         "some-action",
					TaskID:         "some-task",
					DeploymentName: "some-deployment",
					Instance:       "some-instance",
				}),
			}, nil
		}

		shipper = s.NewShipper(
			10*time.Millisecond,
			logger,
			cursor,
			fetcher,
			"dev", "splunk-key", splunkURL,
		)

		httpmock.RegisterResponder(
			"POST", splunkURL,
			func(req *http.Request) (*http.Response, error) {
				body, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())

				var event s.SplunkEvent
				err = json.Unmarshal(body, &event)
				Expect(err).NotTo(HaveOccurred())

				Expect(event).To(MatchAllFields(Fields{
					"SourceType": Equal("bosh-audit-event"),
					"Source":     Equal("dev"),
					"Event": MatchAllFields(Fields{
						"ID":             Equal("abcd"),
						"Timestamp":      BeAssignableToTypeOf(int64(0)),
						"User":           Equal("some-user"),
						"Action":         Equal("some-action"),
						"TaskID":         Equal("some-task"),
						"DeploymentName": Equal("some-deployment"),
						"Instance":       Equal("some-instance"),
					}),
				}))

				return httpmock.NewJsonResponse(200, map[string]interface{}{
					"message": "success",
				})
			},
		)

		var (
			shipError error
			shipWG    sync.WaitGroup
		)

		shipContext, cancelShip := context.WithTimeout(
			context.Background(), 100*time.Millisecond,
		)

		By("running the shipper")
		shipWG.Add(1)
		go func() {
			defer GinkgoRecover()
			shipError = shipper.Run(shipContext)
			shipWG.Done()
		}()

		By("waiting for events to be shipped")
		Eventually(
			httpmock.GetTotalCallCount, "1000ms", "1ms",
		).Should(BeNumerically("==", 5))

		Expect(shipError).NotTo(HaveOccurred())

		By("checking the cursor was updated")
		Expect(cursor.GetTime()).To(BeTemporally("~", time.Unix(5+1, 0)))

		By("cleaning up")
		cancelShip()
		shipWG.Wait()
		Expect(shipError).NotTo(HaveOccurred())
	})
})
