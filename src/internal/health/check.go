package health

import (
	"fmt"
	"net"
	"net/http"
)

func Check(client *http.Client, metricsAddr string) error {
	endpoint, err := Endpoint(metricsAddr)
	if err != nil {
		return err
	}
	response, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func Endpoint(metricsAddr string) (string, error) {
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}
	_, port, err := net.SplitHostPort(metricsAddr)
	if err != nil || port == "" {
		return "", fmt.Errorf("invalid Prometheus address %q", metricsAddr)
	}
	return "http://127.0.0.1:" + port + "/healthz", nil
}
