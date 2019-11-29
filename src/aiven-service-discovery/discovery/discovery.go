package discovery

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"time"

	"code.cloudfoundry.org/lager"
	aiven "github.com/aiven/aiven-go-client"
)

const (
	dnsLoopupParallelism = 5
	userAgent            = "govuk-paas-aiven-service-discovery"
)

type DiscoveredService struct {
	ServiceName string
	ServiceType string
	Plan        string
	Hostname    string
	NodeCount   int
	Targets     []net.IP
}

type PrometheusStaticConfigLabels struct {
	ServiceName string `json:"aiven_service_name"`
	ServiceType string `json:"aiven_service_type"`
	Hostname    string `json:"aiven_hostname"`
	Plan        string `json:"aiven_plan"`
	NodeCount   string `json:"aiven_node_count"`
}

type PrometheusStaticConfig struct {
	Targets []net.IP                     `json:"targets"`
	Labels  PrometheusStaticConfigLabels `json:"labels"`
}

type Discoverer struct {
	AivenAPIToken string
	AivenProject  string

	Interval   time.Duration
	TargetPath string

	Logger lager.Logger
}

func (d *Discoverer) Discover() []DiscoveredService {
	discoveredServices := make([]DiscoveredService, 0)

	lsession := d.Logger.Session("discover")
	lsession.Info("begin")
	defer lsession.Info("end")

	aivenClient, err := aiven.NewTokenClient(d.AivenAPIToken, userAgent)
	if err != nil {
		lsession.Error("err-aiven-new-token-client", err, lager.Data{})
		return discoveredServices
	}

	services, err := aivenClient.Services.List(d.AivenProject)
	if err != nil {
		lsession.Error(
			"err-aiven-services-list",
			err, lager.Data{},
		)
		return discoveredServices
	}

	lsession = lsession.WithData(lager.Data{"service-count": len(services)})
	lsession.Info("found-services")

	for _, s := range services {
		serviceLogger := lsession.WithData(lager.Data{
			"service": s.Name,
			"type":    s.Type,
		})

		serviceLogger.Info("resolve")

		if s.Type != "elasticsearch" {
			serviceLogger.Info("resolve-skip")
			continue
		}

		hostname, err := s.Hostname()
		if err != nil {
			serviceLogger.Error("err-aiven-service-hostname", err)
			serviceLogger.Info("resolve-skip")
			continue
		}

		ips, err := net.LookupIP(hostname)
		if err != nil {
			serviceLogger.Error("err-resolve-hostname", err)
			serviceLogger.Info("resolve-skip")
			continue
		}
		serviceLogger = serviceLogger.WithData(lager.Data{"ips": ips})

		discoveredServices = append(discoveredServices, DiscoveredService{
			ServiceName: s.Name,
			ServiceType: s.Type,
			Plan:        s.Plan,
			Hostname:    hostname,
			NodeCount:   s.NodeCount,
			Targets:     ips,
		})

		serviceLogger.Info("resolved")
	}

	return discoveredServices
}

func (d *Discoverer) Write(services []DiscoveredService) {
	configs := make([]PrometheusStaticConfig, 0)

	lsession := d.Logger.Session("write")
	lsession.Info("begin")
	defer lsession.Info("end")

	for _, service := range services {
		configs = append(configs, PrometheusStaticConfig{
			Targets: service.Targets,

			Labels: PrometheusStaticConfigLabels{
				ServiceName: service.ServiceName,
				ServiceType: service.ServiceType,

				Hostname: service.Hostname,
				Plan:     service.Plan,

				NodeCount: string(service.NodeCount),
			},
		})
	}

	configsAsJSON, err := json.Marshal(configs)
	if err != nil {
		lsession.Error(
			"err-marshal-json-configs",
			err, lager.Data{"configs": configs},
		)
		return
	}

	err = ioutil.WriteFile(d.TargetPath, configsAsJSON, 0644)
	if err != nil {
		lsession.Error(
			"err-write-json-configs",
			err, lager.Data{"configs": configs, "target": d.TargetPath},
		)
		return
	}
}

func (d *Discoverer) Loop() {
	d.Logger = d.Logger.WithData(lager.Data{"project": d.AivenProject})

	ticker := time.NewTicker(d.Interval)

	for {
		select {
		case <-ticker.C:
			services := d.Discover()
			d.Write(services)
		}
	}
}
