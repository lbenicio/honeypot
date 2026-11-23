package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lbenicio/fakessh/src/internal/health"
)

var processExit = os.Exit
var cliOutput io.Writer = os.Stdout
var cliErrorOutput io.Writer = os.Stderr
var healthCheckClient = &http.Client{Timeout: 2 * time.Second}
var runHealthCheckAt = func(metricsAddr string) error {
	return health.Check(healthCheckClient, metricsAddr)
}
var runHealthCheck = func() error {
	return runHealthCheckAt(os.Getenv("PROMETHEUS_ADDR"))
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		var err error
		if len(os.Args) == 2 {
			err = runHealthCheck()
		} else {
			options, helpRequested, parseErr := parseCLIOptions(os.Args[2:], os.Getenv("PROMETHEUS_ADDR"), cliOutput, cliErrorOutput)
			if parseErr != nil {
				log.Println(parseErr)
				processExit(2)
				return
			}
			if helpRequested {
				return
			}
			err = runHealthCheckAt(options.metricsAddr)
		}
		if err != nil {
			log.Println(err)
			processExit(1)
			return
		}
		return
	}
	options, helpRequested, err := parseCLIOptions(os.Args[1:], os.Getenv("PROMETHEUS_ADDR"), cliOutput, cliErrorOutput)
	if err != nil {
		log.Println(err)
		processExit(2)
		return
	}
	if helpRequested {
		return
	}
	if err := runApplication(options.metricsAddr, options.logFile); err != nil {
		log.Println(err)
	}
}
