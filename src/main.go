package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lbenicio/fakessh/src/internal/health"
)

var processExit = os.Exit
var healthCheckClient = &http.Client{Timeout: 2 * time.Second}
var runHealthCheck = func() error {
	return health.Check(healthCheckClient, os.Getenv("PROMETHEUS_ADDR"))
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthCheck(); err != nil {
			log.Println(err)
			processExit(1)
			return
		}
		return
	}
	if err := runApplication(os.Args[1:], os.Getenv("PROMETHEUS_ADDR")); err != nil {
		log.Println(err)
	}
}
