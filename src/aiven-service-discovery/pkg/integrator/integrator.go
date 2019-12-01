package integrator

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"
	"github.com/prometheus/client_golang/prometheus"

	f "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher"
)

const (
	defaultInterval = 15 * time.Second
	userAgent       = "govuk-paas-aiven-service-discovery-integrator"
)

type Integrator interface {
	Start()
	Stop()

	SetInterval(time.Duration)
}

type integrator struct {
	aivenProject              string
	aivenClient               aiven.Client
	aivenPrometheusEndpointID string

	fetcher f.Fetcher

	logger   lager.Logger
	registry *prometheus.Registry

	stop chan struct{}
	wg   sync.WaitGroup

	interval time.Duration
}

func NewIntegrator(
	aivenProject string,
	aivenAPIToken string,
	aivenPrometheusEndpointID string,

	fetcher f.Fetcher,

	logger lager.Logger,

	registry *prometheus.Registry,
) (Integrator, error) {
	lsession := logger.Session("integrator", lager.Data{"project": aivenProject})

	aivenClient, err := aiven.NewTokenClient(aivenAPIToken, userAgent)
	if err != nil {
		lsession.Error("err-aiven-new-token-client", err)
		return nil, err
	}

	i := integrator{
		aivenProject:              aivenProject,
		aivenClient:               *aivenClient,
		aivenPrometheusEndpointID: aivenPrometheusEndpointID,

		fetcher: fetcher,

		logger:   lsession,
		registry: registry,

		stop: make(chan struct{}),

		interval: defaultInterval,
	}

	return &i, nil
}

func (i *integrator) integrateService(s aiven.Service) {
	lsession := i.logger.Session(
		"integrate-service", lager.Data{"service": s.Name},
	)
	lsession.Info("begin")
	defer lsession.Info("end")

	_, err := i.aivenClient.ServiceIntegrations.Create(
		i.aivenProject,
		aiven.CreateServiceIntegrationRequest{
			DestinationEndpointID: &i.aivenPrometheusEndpointID,
			SourceService:         &s.Name,
			IntegrationType:       "prometheus",
		},
	)

	if err != nil {
		lsession.Error("err-aiven-create-service-integration", err)
	}
}

func (i *integrator) integrate() {
	lsession := i.logger.Session("integrate")
	lsession.Info("begin")
	defer lsession.Info("end")

	services := i.fetcher.Services()

	servicesWithoutPrometheus := make([]aiven.Service, 0)
	for _, service := range services {
		needsPrometheus := true
		for _, integration := range service.Integrations {
			if integration.IntegrationType == "prometheus" {
				needsPrometheus = false
			}
		}

		if needsPrometheus {
			servicesWithoutPrometheus = append(servicesWithoutPrometheus, service)
		}
	}

	eligibleServices := make([]aiven.Service, 0)
	for _, service := range servicesWithoutPrometheus {
		switch service.Type {
		case "elasticsearch":
			eligibleServices = append(eligibleServices, service)
		}
	}

	for _, service := range eligibleServices {
		i.integrateService(service)
	}
}

func (i *integrator) loop() {
	lsession := i.logger.Session("loop")
	lsession.Info("begin")
	defer lsession.Info("end")

	ticker := time.NewTicker(i.interval)
	i.wg.Add(1)

	for {
		select {
		case <-ticker.C:
			i.integrate()
		case <-i.stop:
			i.wg.Done()
			return
		}
	}
}

func (i *integrator) Start() {
	lsession := i.logger.Session("start")
	lsession.Info("begin")
	defer lsession.Info("end")

	go i.loop()
}

func (i *integrator) Stop() {
	lsession := i.logger.Session("stop")
	lsession.Info("begin")
	defer lsession.Info("end")

	close(i.stop)
	i.wg.Wait()
}

func (i *integrator) SetInterval(interval time.Duration) {
	i.interval = interval
}
