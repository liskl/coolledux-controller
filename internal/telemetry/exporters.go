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
// exporters want host:port) and returns the host plus a flag indicating
// whether the scheme implies TLS. Empty endpoints return the zero value
// so the caller can treat "use SDK default" as a distinct case.
func endpointFor(top string, signal config.OTelSignalConfig) (endpoint string, schemeTLS bool) {
	raw := signal.Endpoint
	if raw == "" {
		raw = top
	}
	if raw == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(raw, "https://"):
		return strings.TrimPrefix(raw, "https://"), true
	case strings.HasPrefix(raw, "http://"):
		return strings.TrimPrefix(raw, "http://"), false
	default:
		return raw, false
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
	endpoint, schemeTLS := endpointFor(cfg.Endpoint, cfg.Traces)

	var client otlptrace.Client
	switch proto {
	case "grpc":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}
		client = otlptracegrpc.NewClient(opts...)
	case "http":
		opts := []otlptracehttp.Option{otlptracehttp.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
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
	endpoint, schemeTLS := endpointFor(cfg.Endpoint, cfg.Metrics.OTelSignalConfig)

	switch proto {
	case "grpc":
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
		}
		return otlpmetricgrpc.New(ctx, opts...)
	case "http":
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
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
	endpoint, schemeTLS := endpointFor(cfg.Endpoint, cfg.Logs)

	switch proto {
	case "grpc":
		opts := []otlploggrpc.Option{otlploggrpc.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlploggrpc.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
		}
		return otlploggrpc.New(ctx, opts...)
	case "http":
		opts := []otlploghttp.Option{otlploghttp.WithTimeout(effectiveTimeout(cfg))}
		if endpoint != "" {
			opts = append(opts, otlploghttp.WithEndpoint(endpoint))
		}
		if cfg.Insecure && !schemeTLS {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
		}
		return otlploghttp.New(ctx, opts...)
	}
	return nil, fmt.Errorf("unreachable: protocol %q", proto)
}

// When neither `cfg.Insecure` nor an explicit `http://` scheme is set,
// the OTLP exporters default to TLS using the system cert pool. Custom
// cert pools / client certs aren't a current user requirement, so this
// package intentionally doesn't expose a TLS knob.
