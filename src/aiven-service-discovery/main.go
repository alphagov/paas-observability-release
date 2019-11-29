package main

import (
	"flag"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/alphagov/paas-observability-release/src/aiven-service-discovery/discovery"
)

const (
	discoveryLoopInterval = 120 * time.Second
)

var (
	aivenAPIToken string
	aivenProject  string
	targetPath    string
)

func main() {
	flag.StringVar(&aivenAPIToken, "aiven-api-token", "", "Aiven API token use")
	flag.StringVar(&aivenProject, "aiven-project", "", "Aiven project to discover")
	flag.StringVar(&targetPath, "target-path", "", "File path to where targets will be written")
	flag.Parse()

	if aivenAPIToken == "" {
		log.Fatalf("Flag not specified: --aiven-api-token")
	}

	if aivenProject == "" {
		log.Fatalf("Flag not specified: --aiven-project")
	}

	if targetPath == "" {
		log.Fatalf("Flag not specified: --target-path")
	}

	logger := lager.NewLogger("aiven-service-discovery")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	disco := discovery.Discoverer{
		AivenAPIToken: aivenAPIToken,
		AivenProject:  aivenProject,

		Interval:   discoveryLoopInterval,
		TargetPath: targetPath,

		Logger: logger,
	}

	log.Println("Starting loop...")
	disco.Loop()
}
