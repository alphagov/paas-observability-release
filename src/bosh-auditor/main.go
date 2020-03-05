package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	c "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/cursor"
	f "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/fetcher"
	s "github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/shipper"
)

var (
	lookbackDuration     time.Duration
	prometheusListenPort uint

	boshClientID     string
	boshClientSecret string

	boshCACert string
	uaaCACert  string

	boshURL string
	uaaURL  string

	splunkHECEndpoint string
	splunkToken       string

	cursorDir string
	deployEnv string
)

func main() {
	flag.DurationVar(
		&lookbackDuration,
		"lookback-duration", 3*time.Hour,
		"",
	)
	flag.UintVar(
		&prometheusListenPort,
		"prometheus-listen-port", 9275,
		"Port on which prometheus metrics will be exposed via /metrics",
	)

	flag.StringVar(
		&boshClientID,
		"bosh-client-id", "",
		"Client ID used to get a token for BOSH from UAA",
	)
	flag.StringVar(
		&boshClientSecret,
		"bosh-client-secret", "",
		"Client secret used to get a token for BOSH from UAA",
	)

	flag.StringVar(
		&boshCACert,
		"bosh-ca-cert", "",
		"Certificate authority used by the BOSH Director API in PEM format",
	)
	flag.StringVar(
		&uaaCACert,
		"uaa-ca-cert", "",
		"Certificate authority used by UAA in PEM format",
	)

	flag.StringVar(
		&boshURL,
		"bosh-url", "",
		"URL used for BOSH director",
	)
	flag.StringVar(
		&uaaURL,
		"uaa-url", "",
		"URL used for UAA to authenticate with BOSH director",
	)

	flag.StringVar(
		&splunkHECEndpoint,
		"splunk-hec-endpoint", "",
		"Endpoint for Splunk HTTP Event Collector which will receive shipped events",
	)
	flag.StringVar(
		&splunkToken,
		"splunk-token", "",
		"Token for Splunk HTTP Event Collector which will receive shipped events",
	)

	flag.StringVar(
		&cursorDir,
		"cursor-dir", "",
		"Persistent directory in which bosh-auditor stores cursor files",
	)

	flag.StringVar(
		&deployEnv,
		"deploy-env", "",
		"Environment in which bosh-auditor is deployed",
	)

	flag.Parse()

	if 0 == prometheusListenPort || prometheusListenPort > 65535 {
		log.Fatalf("Flag invalid: --prometheus-listen-port must be between 1 and 65535")
	}

	if boshClientID == "" || boshClientSecret == "" {
		log.Fatalf("Flag invalid: --bosh-client-id and --bosh-client-secret must be provided")
	}

	if boshCACert == "" || uaaCACert == "" {
		log.Fatalf("Flag invalid: --bosh-ca-cert and --uaa-ca-cert must be provided")
	}

	if boshURL == "" || uaaURL == "" {
		log.Fatalf("Flag invalid: --bosh-url and --uaa-url must be provided")
	}

	if splunkHECEndpoint == "" || splunkToken == "" {
		log.Fatalf("Flag invalid: --splunk-hec-endpoint and --splunk-token must be provided")
	}

	if cursorDir == "" {
		log.Fatalf("Flag invalid: --cursor-dir must be provided")
	}

	if deployEnv == "" {
		log.Fatalf("Flag invalid: --deploy-env must be provided")
	}

	logger := lager.NewLogger("bosh-auditor")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Info("configured", lager.Data{
		"lookback-duration":      lookbackDuration.String(),
		"prometheus-listen-port": prometheusListenPort,
		"splunk-hec-endpoint":    splunkHECEndpoint,
	})

	ctx, shutdown := context.WithCancel(context.Background())
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		shutdown()
	}()

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", prometheusListenPort),
		Handler: promhttp.Handler(),
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := metricsServer.ListenAndServe()
		if err != nil {
			logger.Error("err-fatal-metrics-server", err)
		}
		shutdown()
		os.Exit(1)
	}()

	wg.Add(1)
	go func() {
		cursor := c.NewFileCursor(
			"bosh-auditor-splunk-shipper",
			cursorDir,
			time.Unix(1451606400, 0),
			logger.Session("bosh-auditor-splunk-shipper-file-cursor"),
		)

		fetcher := f.NewFetcher(
			boshURL,
			uaaURL,
			boshClientID,
			boshClientSecret,
			boshCACert,
			uaaCACert,
		)

		shipper := s.NewShipper(
			20*time.Second,
			logger.Session("bosh-auditor-splunk-shipper"),
			cursor,
			fetcher,
			deployEnv,
			splunkToken,
			splunkHECEndpoint,
		)

		err := shipper.Run(ctx)
		if err != nil {
			logger.Error("err-fatal-shipper", err)
		}
		shutdown()
		os.Exit(1)
	}()

	wg.Wait()
}
