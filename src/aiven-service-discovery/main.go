package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"code.cloudfoundry.org/lager"

	d "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/discoverer"
	f "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/fetcher"
	i "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/integrator"
	r "github.com/alphagov/paas-observability-release/src/aiven-service-discovery/pkg/resolver"
)

const (
	discoveryLoopInterval = 120 * time.Second
)

var (
	aivenAPIToken              string
	aivenProject               string
	aivenPrometheusEndpointID  string
	serviceDiscoveryTargetPath string
)

func main() {
	flag.StringVar(&aivenAPIToken, "aiven-api-token", "", "Aiven API token use")
	flag.StringVar(&aivenProject, "aiven-project", "", "Aiven project to discover")
	flag.StringVar(&aivenPrometheusEndpointID, "aiven-prometheus-endpoint-id", "", "Aiven Prometheus service integration endpoint to use")
	flag.StringVar(&serviceDiscoveryTargetPath, "service-discovery-target-path", "", "File path to where targets will be written")
	flag.Parse()

	if aivenAPIToken == "" {
		log.Fatalf("Flag not specified: --aiven-api-token")
	}

	if aivenProject == "" {
		log.Fatalf("Flag not specified: --aiven-project")
	}

	if aivenPrometheusEndpointID == "" {
		log.Fatalf("Flag not specified: --aiven-prometheus-endpoint-id")
	}

	if serviceDiscoveryTargetPath == "" {
		log.Fatalf("Flag not specified: --service-discovery-target-path")
	}

	logger := lager.NewLogger("aiven-service-discovery")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	fetcher, err := f.NewFetcher(
		aivenProject, aivenAPIToken,
		logger,
	)
	if err != nil {
		log.Fatalf("Could not create fetcher: %s", err)
	}

	integrator, err := i.NewIntegrator(
		aivenProject, aivenAPIToken, aivenPrometheusEndpointID,
		fetcher,
		logger,
	)
	if err != nil {
		log.Fatalf("Could not create integrator: %s", err)
	}

	discoverer, err := d.NewDiscoverer(
		aivenProject, serviceDiscoveryTargetPath,
		fetcher, r.NewResolver(),
		logger,
	)
	if err != nil {
		log.Fatalf("Could not create discoverer: %s", err)
	}

	fetcher.Start()
	integrator.Start()
	discoverer.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt

	fetcher.Stop()
	integrator.Stop()
	discoverer.Stop()
}
