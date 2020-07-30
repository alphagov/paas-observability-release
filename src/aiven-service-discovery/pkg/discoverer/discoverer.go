package discoverer

import (
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"

	f "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher"
	r "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/resolver"
	w "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/writer"
)

func init() {
	initMetrics()
}

const (
	defaultInterval         = 45 * time.Second
	dnsDiscoveryConcurrency = 5
)

type Discoverer interface {
	Start()
	Stop()

	SetInterval(time.Duration)
}

type discoverer struct {
	aivenProject string

	fetcher  f.Fetcher
	resolver r.Resolver
	writer   w.Writer

	logger lager.Logger

	stop chan struct{}
	wg   sync.WaitGroup

	interval time.Duration
}

func NewDiscoverer(
	aivenProject string,
	fetcher f.Fetcher,
	resolver r.Resolver,
	writer w.Writer,

	logger lager.Logger,
) (Discoverer, error) {
	lsession := logger.Session("discoverer", lager.Data{"project": aivenProject})

	d := discoverer{
		aivenProject: aivenProject,

		fetcher:  fetcher,
		resolver: resolver,
		writer:   writer,

		logger: lsession,

		stop: make(chan struct{}),

		interval: defaultInterval,
	}

	return &d, nil
}

func (d *discoverer) goPerformDNSDiscovery(
	services []aiven.Service,
	wg *sync.WaitGroup,
	results chan w.PrometheusTargetConfig,
) {
	defer wg.Done()

	lsession := d.logger.Session("go-perform-dns-discovery")

	for _, service := range services {

		DiscovererDNSDiscoveriesTotal.Inc()

		hostname, err := service.Hostname()
		if err != nil {
			lsession.Error(
				"err-aiven-get-hostname", err, lager.Data{"service": service.Name},
			)

			DiscovererDNSDiscoveryErrorsTotal.Inc()

			continue
		}

		ips, err := d.resolver.Resolve(hostname)
		if err != nil {
			lsession.Error(
				"err-resolve", err, lager.Data{"service": service.Name},
			)

			DiscovererDNSDiscoveryErrorsTotal.Inc()

			continue
		}

		results <- w.PrometheusTargetConfig{
			Labels: w.PrometheusTargetConfigLabels{
				ServiceName: service.Name,
				ServiceType: service.Type,
				Hostname:    hostname,
				Plan:        service.Plan,
				Cloud:       service.CloudName,
				NodeCount:   fmt.Sprintf("%d", service.NodeCount),
			},
			Targets: ips,
		}
	}
}

func (d *discoverer) performDNSDiscovery(services []aiven.Service) []w.PrometheusTargetConfig {
	lsession := d.logger.Session("perform-dns-discovery")
	lsession.Info("begin")
	defer lsession.Info("end")

	work := make(map[int][]aiven.Service, 0)
	for index, service := range services {
		targetQueue := index % dnsDiscoveryConcurrency
		work[targetQueue] = append(work[targetQueue], service)
	}

	var wg sync.WaitGroup
	results := make(chan w.PrometheusTargetConfig, len(services))

	for _, queue := range work {
		wg.Add(1)
		go d.goPerformDNSDiscovery(queue, &wg, results)
	}

	wg.Wait()
	close(results)

	targets := make([]w.PrometheusTargetConfig, 0)
	for target := range results {
		targets = append(targets, target)
	}

	return targets
}

func (d *discoverer) discoverAndWrite() {
	lsession := d.logger.Session("discover")
	lsession.Info("begin")
	defer lsession.Info("end")

	services := d.fetcher.Services()

	servicesWithPrometheus := make([]aiven.Service, 0)
	for _, service := range services {
		hasPrometheus := false
		for _, integration := range service.Integrations {
			if integration.IntegrationType == "prometheus" {
				hasPrometheus = true
			}
		}

		if hasPrometheus {
			servicesWithPrometheus = append(servicesWithPrometheus, service)
		}
	}

	targets := d.performDNSDiscovery(servicesWithPrometheus)
	lsession.Info("targets", lager.Data{"targets": targets})
	DiscovererWriteTargetsTotal.Inc()
	d.writer.Write(targets)
}

func (d *discoverer) loop() {
	lsession := d.logger.Session("loop")
	lsession.Info("begin")
	defer lsession.Info("end")

	d.wg.Add(1)

	for {
		select {
		case <-time.After(d.interval):
			d.discoverAndWrite()
		case <-d.stop:
			d.wg.Done()
			return
		}
	}
}

func (d *discoverer) Start() {
	lsession := d.logger.Session("start")
	lsession.Info("begin")
	defer lsession.Info("end")

	go d.loop()
}

func (d *discoverer) Stop() {
	lsession := d.logger.Session("stop")
	lsession.Info("begin")
	defer lsession.Info("end")

	close(d.stop)
	d.wg.Wait()
}

func (d *discoverer) SetInterval(interval time.Duration) {
	d.interval = interval
}
