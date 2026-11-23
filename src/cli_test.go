package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLIOptions(t *testing.T) {
	t.Run("uses environment metrics default", func(t *testing.T) {
		var output, errorOutput bytes.Buffer
		got, help, err := parseCLIOptions(nil, "127.0.0.1:9211", &output, &errorOutput)
		if err != nil {
			t.Fatalf("parseCLIOptions() error = %v", err)
		}
		if help {
			t.Fatal("help requested for empty arguments")
		}
		if got.metricsAddr != "127.0.0.1:9211" || got.logFile != "" {
			t.Errorf("options = %+v, want env metrics address and no log file", got)
		}
	})

	t.Run("uses default metrics address", func(t *testing.T) {
		var output, errorOutput bytes.Buffer
		got, _, err := parseCLIOptions(nil, "", &output, &errorOutput)
		if err != nil {
			t.Fatalf("parseCLIOptions() error = %v", err)
		}
		if got.metricsAddr != defaultMetricsAddress {
			t.Errorf("metrics address = %q, want %q", got.metricsAddr, defaultMetricsAddress)
		}
	})

	t.Run("parses metrics and log file flags", func(t *testing.T) {
		var output, errorOutput bytes.Buffer
		got, help, err := parseCLIOptions([]string{"--metrics", "127.0.0.1:9211", "--log-file=/var/log/fakessh.log"}, "", &output, &errorOutput)
		if err != nil {
			t.Fatalf("parseCLIOptions() error = %v", err)
		}
		if help {
			t.Fatal("help requested when parsing configuration flags")
		}
		if got.metricsAddr != "127.0.0.1:9211" || got.logFile != "/var/log/fakessh.log" {
			t.Errorf("options = %+v, want configured metrics and log file", got)
		}
	})

	t.Run("prints complete help", func(t *testing.T) {
		var output, errorOutput bytes.Buffer
		_, help, err := parseCLIOptions([]string{"-h"}, "", &output, &errorOutput)
		if err != nil {
			t.Fatalf("parseCLIOptions() error = %v", err)
		}
		if !help {
			t.Fatal("help was not reported")
		}
		for _, want := range []string{"Usage:", "--metrics", "--log-file", "healthcheck", "Prometheus metrics", "Logs go to stderr"} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("help output is missing %q: %q", want, output.String())
			}
		}
	})

	t.Run("rejects unknown flags and positional arguments", func(t *testing.T) {
		for _, args := range [][]string{{"--unknown"}, {"old-log-directory"}} {
			var output, errorOutput bytes.Buffer
			if _, _, err := parseCLIOptions(args, "", &output, &errorOutput); err == nil {
				t.Errorf("parseCLIOptions(%v) succeeded, want error", args)
			}
		}
	})
}
