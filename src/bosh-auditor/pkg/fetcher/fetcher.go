package fetcher

import (
	"fmt"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"time"
)

type Fetcher func(time.Time) ([]boshdir.Event, error)

type fetcher struct {
	boshCACert       string
	uaaCACert        string
	boshClientID     string
	boshClientSecret string
	uaaURL           string
	boshURL          string
}

func NewFetcher(
	boshURL string,
	uaaURL string,
	boshClientID string,
	boshClientSecret string,
	boshCACert string,
	uaaCACert string,
) Fetcher {
	return func(t time.Time) ([]boshdir.Event, error) {
		logger := boshlog.NewLogger(boshlog.LevelError)
		uaaFactory := boshuaa.NewFactory(logger)

		uaaConfig, err := boshuaa.NewConfigFromURL(uaaURL)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		uaaConfig.Client = boshClientID
		uaaConfig.ClientSecret = boshClientSecret
		uaaConfig.CACert = uaaCACert

		uaa, err := uaaFactory.New(uaaConfig)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		boshFactory := boshdir.NewFactory(logger)

		boshConfig, err := boshdir.NewConfigFromURL(boshURL)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		boshConfig.CACert = boshCACert
		boshConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc

		bosh, err := boshFactory.New(boshConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		return bosh.Events(boshdir.EventsFilter{
			After: t.Format(time.RFC3339),
		})
	}
}
