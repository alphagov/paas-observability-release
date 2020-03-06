package shipper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"

	c "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/cursor"
	f "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/fetcher"
)

type BoshEvent struct {
	ID             string `json:"id"`
	Timestamp      int64  `json:"timestamp"`
	User           string `json:"user"`
	Action         string `json:"action"`
	TaskID         string `json:"task"`
	DeploymentName string `json:"deployment"`
	Instance       string `json:"instance"`
}

type SplunkEvent struct {
	SourceType string    `json:"sourcetype"`
	Source     string    `json:"source"`
	Event      BoshEvent `json:"event"`
}

type splunkHTTPClient struct {
	client       http.Client
	splunkAPIKey string
}

func (c *splunkHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", c.splunkAPIKey))
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

type Shipper interface {
	Run(context.Context) error
}

type shipper struct {
	schedule  time.Duration
	logger    lager.Logger
	cursor    c.Cursor
	fetcher   f.Fetcher
	deployEnv string
	client    *httpclient.Client
	splunkURL string

	eventsShipped int
}

func NewShipper(
	schedule time.Duration,
	logger lager.Logger,
	cursor c.Cursor,
	fetcher f.Fetcher,
	deployEnv string,
	splunkAPIKey string,
	splunkURL string,
) Shipper {
	logger = logger.Session("bosh-events-to-splunk-shipper")

	var (
		requestTimeout         = 2 * time.Second
		initalTimeout          = 100 * time.Millisecond
		maxTimeout             = 2 * time.Second
		exponent       float64 = 2
		jitter                 = 500 * time.Millisecond
		maxRetries             = 3

		backoff = heimdall.NewExponentialBackoff(
			initalTimeout, maxTimeout,
			exponent, jitter,
		)

		retrier = heimdall.NewRetrier(backoff)
	)

	client := httpclient.NewClient(
		httpclient.WithHTTPClient(&splunkHTTPClient{
			client:       *http.DefaultClient,
			splunkAPIKey: splunkAPIKey,
		}),
		httpclient.WithHTTPTimeout(requestTimeout),
		httpclient.WithRetrier(retrier),
		httpclient.WithRetryCount(maxRetries),
	)

	return &shipper{
		schedule,
		logger,
		cursor,
		fetcher,
		deployEnv,
		client,
		splunkURL,
		0,
	}
}

func convertEvent(event boshdir.Event) BoshEvent {
	return BoshEvent{
		ID:             event.ID(),
		Timestamp:      event.Timestamp().Unix(),
		User:           event.User(),
		Action:         event.Action(),
		TaskID:         event.TaskID(),
		DeploymentName: event.DeploymentName(),
		Instance:       event.Instance(),
	}
}

func (s *shipper) shipEvent(event boshdir.Event) error {
	bytesToShip, err := json.Marshal(SplunkEvent{
		SourceType: "bosh-audit-event",
		Source:     s.deployEnv,
		Event:      convertEvent(event),
	})

	if err != nil {
		return err
	}

	resp, err := s.client.Post(
		s.splunkURL,
		bytes.NewReader(bytesToShip),
		http.Header{},
	)

	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if err != nil {
		return err
	}

	if 200 <= resp.StatusCode && resp.StatusCode < 300 {
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	return fmt.Errorf("Status: %d Body: %s", resp.StatusCode, body)
}

func (s *shipper) Run(ctx context.Context) error {
	lsession := s.logger.Session("run")

	lsession.Info("start")
	defer lsession.Info("end")

	for {
		select {
		case <-ctx.Done():
			lsession.Info("done")
			return nil
		case <-time.After(s.schedule):
			startTime := time.Now()

			latestEventTimestamp := s.cursor.GetTime()
			eventsToShip, err := s.fetcher(latestEventTimestamp)

			if err != nil {
				lsession.Error("err-get-unshipped-bosh-audit-events-for-shipper", err)
				continue
			}

			var (
				shippedEvents    = make([]boshdir.Event, 0)
				allEventsShipped = true
			)

			for _, event := range eventsToShip {
				err := s.shipEvent(event)

				if err != nil {
					lsession.Error("err-ship-event", err)
					allEventsShipped = false
					break
				}

				if event.Timestamp().After(latestEventTimestamp) {
					latestEventTimestamp = event.Timestamp()
				}

				shippedEvents = append(shippedEvents, event)
				s.eventsShipped++
				EventsShippedTotal.Inc()
			}

			if err = s.cursor.UpdateTime(latestEventTimestamp); err != nil {
				lsession.Error("err-update-shipper-cursor", err)
			}

			duration := time.Since(startTime)
			lsession.Info(
				"shipped-events",
				lager.Data{
					"duration":             duration,
					"events-shipped":       len(shippedEvents),
					"total-events-shipped": s.eventsShipped,
					"all-events-shipped":   allEventsShipped,
				},
			)
		}
	}
}
