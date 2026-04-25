package telemetry

import (
	"testing"

	"github.com/liskl/coolledux-controller/internal/config"
)

func TestEndpointFor_StripsScheme(t *testing.T) {
	tests := []struct {
		name       string
		top        string
		signal     config.OTelSignalConfig
		wantHost   string
		wantScheme string
	}{
		{"empty", "", config.OTelSignalConfig{}, "", ""},
		{"top no scheme", "host:4317", config.OTelSignalConfig{}, "host:4317", ""},
		{"top http", "http://host:4318", config.OTelSignalConfig{}, "host:4318", "http"},
		{"top https", "https://host:4318", config.OTelSignalConfig{}, "host:4318", "https"},
		{"signal overrides top", "top:4317", config.OTelSignalConfig{Endpoint: "http://override:4318"}, "override:4318", "http"},
		{"signal blank inherits top", "top:4317", config.OTelSignalConfig{}, "top:4317", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, scheme := endpointFor(tt.top, tt.signal)
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if scheme != tt.wantScheme {
				t.Errorf("scheme = %q, want %q", scheme, tt.wantScheme)
			}
		})
	}
}

// TestShouldUseInsecure_SchemeOverridesConfigFlag is the regression test
// for the bug Copilot caught: an explicit http:// scheme used to leave
// TLS attempted whenever cfg.Insecure was false.
func TestShouldUseInsecure_SchemeOverridesConfigFlag(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.OTelConfig
		scheme   string
		want     bool
		intent   string
	}{
		{"http scheme + insecure=false → still insecure", &config.OTelConfig{Insecure: false}, "http", true,
			"the user typed http://, that's a clear request to skip TLS"},
		{"http scheme + insecure=true → insecure", &config.OTelConfig{Insecure: true}, "http", true,
			"both signals agree: plaintext"},
		{"https scheme + insecure=true → TLS wins", &config.OTelConfig{Insecure: true}, "https", false,
			"https:// is an explicit TLS request even if config says insecure"},
		{"https scheme + insecure=false → TLS", &config.OTelConfig{Insecure: false}, "https", false,
			"both signals agree: TLS"},
		{"no scheme + insecure=true → insecure", &config.OTelConfig{Insecure: true}, "", true,
			"bare host:port falls through to config flag"},
		{"no scheme + insecure=false → TLS (system certs)", &config.OTelConfig{Insecure: false}, "", false,
			"bare host:port falls through to config flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseInsecure(tt.cfg, tt.scheme); got != tt.want {
				t.Errorf("shouldUseInsecure(insecure=%v, scheme=%q) = %v, want %v\nintent: %s",
					tt.cfg.Insecure, tt.scheme, got, tt.want, tt.intent)
			}
		})
	}
}

func TestProtocolFor(t *testing.T) {
	tests := []struct {
		name    string
		top     string
		signal  config.OTelSignalConfig
		want    string
		wantErr bool
	}{
		{"defaults to grpc", "", config.OTelSignalConfig{}, "grpc", false},
		{"top grpc", "grpc", config.OTelSignalConfig{}, "grpc", false},
		{"top http", "http", config.OTelSignalConfig{}, "http", false},
		{"signal overrides top", "grpc", config.OTelSignalConfig{Protocol: "http"}, "http", false},
		{"unknown rejected", "carrier-pigeon", config.OTelSignalConfig{}, "", true},
		{"signal unknown rejected", "grpc", config.OTelSignalConfig{Protocol: "smoke"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := protocolFor(tt.top, tt.signal)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
