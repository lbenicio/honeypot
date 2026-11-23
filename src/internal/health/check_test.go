package health

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		metricsAddr string
		want        string
		wantErr     bool
	}{
		{name: "default address", want: "http://127.0.0.1:2112/healthz"},
		{name: "custom address", metricsAddr: "0.0.0.0:9211", want: "http://127.0.0.1:9211/healthz"},
		{name: "malformed address", metricsAddr: "invalid", wantErr: true},
		{name: "missing port", metricsAddr: ":", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Endpoint(test.metricsAddr)
			if test.wantErr {
				if err == nil {
					t.Fatal("Endpoint() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if got != test.want {
				t.Errorf("Endpoint() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	transportError := errors.New("connection refused")
	tests := []struct {
		name        string
		metricsAddr string
		statusCode  int
		err         error
		wantErr     string
	}{
		{name: "healthy response", metricsAddr: "127.0.0.1:9211", statusCode: http.StatusOK},
		{name: "unhealthy response", metricsAddr: "127.0.0.1:9211", statusCode: http.StatusServiceUnavailable, wantErr: "healthcheck returned HTTP 503"},
		{name: "transport error", metricsAddr: "127.0.0.1:9211", err: transportError, wantErr: "connection refused"},
		{name: "invalid metrics address", metricsAddr: "invalid", wantErr: "invalid Prometheus address"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "http://127.0.0.1:9211/healthz" {
					t.Errorf("healthcheck URL = %q", request.URL)
				}
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{
					StatusCode: test.statusCode,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    request,
				}, nil
			})}
			err := Check(client, test.metricsAddr)
			if test.wantErr == "" && err != nil {
				t.Fatalf("Check() error = %v, want nil", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("Check() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
