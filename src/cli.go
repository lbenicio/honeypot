package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const defaultMetricsAddress = ":2112"

type cliOptions struct {
	metricsAddr string
	logFile     string
	help        bool
}

func parseCLIOptions(args []string, metricsDefault string, output, errorOutput io.Writer) (cliOptions, bool, error) {
	if metricsDefault == "" {
		metricsDefault = defaultMetricsAddress
	}

	options := cliOptions{}
	flags := flag.NewFlagSet("fakessh", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	flags.StringVar(&options.metricsAddr, "metrics", metricsDefault, "Prometheus listen address; serves /metrics and /healthz")
	flags.StringVar(&options.logFile, "log-file", "", "Write logs to this file (default: stderr)")
	flags.BoolVar(&options.help, "help", false, "Print usage, flags, and operating messages")
	flags.BoolVar(&options.help, "h", false, "Print usage, flags, and operating messages")
	flags.Usage = func() {
		_, _ = io.WriteString(output, "FakeSSH runs an SSH honeypot that records login attempts and rejects password authentication.\n\nUsage:\n  fakessh [flags]\n  fakessh healthcheck [flags]\n\nFlags:\n")
		flags.VisitAll(func(f *flag.Flag) {
			if f.Name == "h" {
				return
			}
			name := "--" + f.Name
			if f.Name == "help" {
				name = "-h, --help"
			} else {
				name += " string"
			}
			_, _ = fmt.Fprintf(output, "  %-24s %s", name, f.Usage)
			if f.DefValue != "" && f.DefValue != "false" {
				_, _ = fmt.Fprintf(output, " (default %q)", f.DefValue)
			}
			_, _ = io.WriteString(output, "\n")
		})
		_, _ = io.WriteString(output, "\nMessages:\n  SSH listens on TCP port 22.\n  Prometheus metrics are exported at /metrics; /healthz reports SSH readiness.\n  The metrics address defaults to PROMETHEUS_ADDR, then :2112.\n  Logs go to stderr unless --log-file is set.\n")
	}

	if err := flags.Parse(args); err != nil {
		return options, false, err
	}
	if options.help {
		flags.Usage()
		return options, true, nil
	}
	if flags.NArg() > 0 {
		return options, false, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	return options, false, nil
}
