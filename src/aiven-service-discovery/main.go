package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	serviceNamesFile           string
	prometheusListenPort       uint
	logLevel                   int
)

func main() {
	flag.StringVar(&aivenAPIToken, "aiven-api-token", "", "Aiven API token use")
	flag.StringVar(&aivenProject, "aiven-project", "", "Aiven project to discover")
	flag.StringVar(&aivenPrometheusEndpointID, "aiven-prometheus-endpoint-id", "", "Aiven Prometheus service integration endpoint to use")
	flag.StringVar(&serviceDiscoveryTargetPath, "service-discovery-target-path", "", "File path to where targets will be written")
	flag.StringVar(&serviceNamesFile, "service-names-file", "", "File path where the names of services to scrape lives")
	flag.UintVar(&prometheusListenPort, "prometheus-listen-port", 9274, "Port on which prometheus metrics will be exposed via /metrics")
	flag.IntVar(&logLevel, "log-level", int(lager.INFO), "Log level")
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

	if 0 == prometheusListenPort || prometheusListenPort > 65535 {
		log.Fatalf("Flag invalid: --prometheus-listen-port must be between 1 and 65535")
	}

	logger := lager.NewLogger("aiven-service-discovery")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.LogLevel(logLevel)))

	fetcher, err := f.NewFetcher(
		aivenProject, aivenAPIToken,
		logger,
	)
	if err != nil {
		log.Fatalf("Could not create fetcher: %s", err)
	}

	if serviceNamesFile != "" {
		content, err := ioutil.ReadFile(serviceNamesFile)
		if err != nil {
			log.Fatalf("Could not read file at path given by --service-names-file=%s. Error reading file: %s", serviceNamesFile, err)
		}

		lines := strings.Split(string(content), "\n")
		fetcher = f.NewFilteredFetcher(fetcher, lines, logger)
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

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", prometheusListenPort),
		Handler: promhttp.Handler(),
	}

	go func() {
		err := metricsServer.ListenAndServe()
		if err != nil {
			log.Fatalf("Could not listen and serve metrics: %s", err)
		}
	}()

	fetcher.Start()
	integrator.Start()
	discoverer.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	sig := <-interrupt

	logger.Info("received-signal", lager.Data{"signal": sig})

	fetcher.Stop()
	integrator.Stop()
	discoverer.Stop()
	metricsServer.Shutdown(context.Background())
}
