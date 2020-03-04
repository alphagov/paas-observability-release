package fetcher

import (
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"time"
	"fmt"
)

type Fetcher func(time.Time) (time.Time, []boshdir.Event, error)

type fetcher struct {
	boshCACert       string
	uaaCACert        string
	boshClientID     string
	boshClientSecret string
	uaaURL           string
	boshURL          string
}

func NewFetcher(
	boshCACert string,
	uaaCACert string,
	boshClientID string,
	boshClientSecret string,
	uaaURL string,
	boshURL string,
) Fetcher {
	return func(t time.Time) (time.Time, []boshdir.Event, error) {
		logger := boshlog.NewLogger(boshlog.LevelError)
		uaaFactory := boshuaa.NewFactory(logger)

		uaaConfig, err := boshuaa.NewConfigFromURL(uaaURL)
		if err != nil {
			fmt.Println(err)
			return t, nil, err
		}

		uaaConfig.Client = boshClientID
		uaaConfig.ClientSecret = boshClientSecret
		uaaConfig.CACert = uaaCACert

		uaa, err := uaaFactory.New(uaaConfig)
		if err != nil {
			fmt.Println(err)
			return t, nil, err
		}

		boshFactory := boshdir.NewFactory(logger)

		boshConfig, err := boshdir.NewConfigFromURL(boshURL)
		if err != nil {
			fmt.Println(err)
			return t, nil, err
		}

		boshConfig.CACert = boshCACert
		boshConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc

		bosh, err := boshFactory.New(boshConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
		if err != nil {
			fmt.Println(err)
			return t, nil, err
		}

		events, err := bosh.Events(boshdir.EventsFilter{
			After: t.Format(time.RFC3339),
		})

		if err != nil {
			fmt.Println(err)
			return t, nil, err
		}

		if len(events) == 0 {
			fmt.Println(err)
			return t, events, nil
		}

		latestEventTimestamp := events[0].Timestamp()
		for _, e := range events {
			if e.Timestamp().After(latestEventTimestamp) {
				latestEventTimestamp = e.Timestamp()
			}
		}

		return latestEventTimestamp, events, nil
	}
}
