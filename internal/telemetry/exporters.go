package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/liskl/coolledux-controller/internal/config"
)

// Default timeout applied when the user hasn't set one explicitly.
const defaultExporterTimeout = 10 * time.Second

// protocolFor picks the effective protocol for a signal. Empty signal
// override inherits the top-level protocol, which itself falls back to
// "grpc". Unknown values are rejected.
func protocolFor(top string, signal config.OTelSignalConfig) (string, error) {
	p := signal.Protocol
	if p == "" {
		p = top
	}
	if p == "" {
		p = "grpc"
	}
	switch p {
	case "grpc", "http":
		return p, nil
	default:
		return "", fmt.Errorf("unknown OTLP protocol %q (expected \"grpc\" or \"http\")", p)
	}
}

// endpointFor strips an optional scheme prefix from the endpoint (OTLP
// exporters want host:port) and returns the host plus the scheme it
// found ("", "http", or "https"). The previous bool return was an
// antipattern: with three states encoded as a bool, callers couldn't
// distinguish "no scheme" from "explicit http", which led to TLS being
// attempted against http:// endpoints.
func endpointFor(top string, signal config.OTelSignalConfig) (host, scheme string) {
	raw := signal.Endpoint
	if raw == "" {
		raw = top
	}
	if raw == "" {
		return "", ""
	}
	switch {
	case strings.HasPrefix(raw, "https://"):
		return strings.TrimPrefix(raw, "https://"), "https"
	case strings.HasPrefix(raw, "http://"):
		return strings.TrimPrefix(raw, "http://"), "http"
	default:
		return raw, ""
	}
}

// shouldUseInsecure consolidates the TLS-vs-plaintext decision for a
// given exporter call. An explicit "http://" scheme overrides the
// config flag (a user who wrote "http://" surely doesn't want TLS),
// "https://" forces TLS regardless of the flag, and a scheme-less
// endpoint defers to cfg.Insecure.
func shouldUseInsecure(cfg *config.OTelConfig, scheme string) bool {
	switch scheme {
	case "http":
		return true
	case "https":
		return false
	default:
		return cfg.Insecure
	}
}

func effectiveTimeout(cfg *config.OTelConfig) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return defaultExporterTimeout
}

func newTraceExporter(ctx context.Context, cfg *config.OTelConfig) (*otlptrace.Exporter, error) {
	proto, err := protocolFor(cfg.Protocol, cfg.Traces)
	if err != nil {
		return nil, err
	}
	endpoint, scheme := endpointFor(cfg.Endpoint, cfg.Traces)
	insecure := shouldUseInsecure(cfg, scheme)
	timeout := effectiveTimeout(cfg)

	var client otlptrace.Client
	switch proto {
	case "grpc":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}
		client = otlptracegrpc.NewClient(opts...)
	case "http":
		opts := []otlptracehttp.Option{otlptracehttp.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
		}
		client = otlptracehttp.NewClient(opts...)
	default:
		return nil, fmt.Errorf("unreachable: protocol %q", proto)
	}
	return otlptrace.New(ctx, client)
}

func newMetricExporter(ctx context.Context, cfg *config.OTelConfig) (sdkmetric.Exporter, error) {
	proto, err := protocolFor(cfg.Protocol, cfg.Metrics.OTelSignalConfig)
	if err != nil {
		return nil, err
	}
	endpoint, scheme := endpointFor(cfg.Endpoint, cfg.Metrics.OTelSignalConfig)
	insecure := shouldUseInsecure(cfg, scheme)
	timeout := effectiveTimeout(cfg)

	switch proto {
	case "grpc":
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
		}
		return otlpmetricgrpc.New(ctx, opts...)
	case "http":
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	}
	return nil, fmt.Errorf("unreachable: protocol %q", proto)
}

func newLogExporter(ctx context.Context, cfg *config.OTelConfig) (sdklog.Exporter, error) {
	proto, err := protocolFor(cfg.Protocol, cfg.Logs)
	if err != nil {
		return nil, err
	}
	endpoint, scheme := endpointFor(cfg.Endpoint, cfg.Logs)
	insecure := shouldUseInsecure(cfg, scheme)
	timeout := effectiveTimeout(cfg)

	switch proto {
	case "grpc":
		opts := []otlploggrpc.Option{otlploggrpc.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlploggrpc.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
		}
		return otlploggrpc.New(ctx, opts...)
	case "http":
		opts := []otlploghttp.Option{otlploghttp.WithTimeout(timeout)}
		if endpoint != "" {
			opts = append(opts, otlploghttp.WithEndpoint(endpoint))
		}
		if insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
		}
		return otlploghttp.New(ctx, opts...)
	}
	return nil, fmt.Errorf("unreachable: protocol %q", proto)
}

// TLS contract: scheme on the endpoint URL takes precedence over
// cfg.Insecure. "http://" forces plaintext, "https://" forces TLS,
// and a bare host:port falls back to cfg.Insecure (which the example
// config ships as true for the typical cluster-internal collector).
// When TLS is in effect the OTLP exporters use the system cert pool;
// custom cert pools / client certs aren't a current user requirement.
