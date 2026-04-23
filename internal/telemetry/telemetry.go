// Package telemetry wires the OpenTelemetry trace, metric, and log
// providers into the coolledux-controller service. When disabled (the
// default), every accessor returns an OTel no-op provider so call sites
// can unconditionally write `tel.Tracer("ble").Start(...)` without
// worrying about nil checks or feature flags.
//
// Enable by setting `otel.enabled: true` in config.yaml and pointing
// `otel.endpoint` at an OTLP-receiving collector. See
// docs/operations/observability.md for the full config reference and the
// homelab cheatsheet.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/liskl/coolledux-controller/internal/config"
)

// Provider owns the OTel SDK providers and their shutdown handles. Pass
// it to subsystems that need to emit telemetry; don't import the OTel
// SDK directly from other packages.
type Provider struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	loggerProvider log.LoggerProvider

	// shutdownFns runs every registered Shutdown in reverse order on
	// Provider.Shutdown. Each fn should be idempotent; the SDK exporter
	// Shutdown methods already are.
	shutdownFns []func(context.Context) error

	// enabled tracks whether the provider was built from a live
	// configuration or is the no-op fallback, so callers can branch on
	// e.g. "should I register a slog bridge?".
	enabled bool

	// stdoutHandler is the existing JSON/text handler main already owns;
	// it flows through as the first destination of SlogHandler() when
	// OTel is enabled. Copied here so callers don't have to thread it
	// through every telemetry call.
	stdoutHandler slog.Handler

	shutdownOnce sync.Once
}

// BuildInfo carries optional service metadata that ends up as OTel
// resource attributes. Populate from `debug.ReadBuildInfo()` in main
// when available; empty fields are dropped.
type BuildInfo struct {
	ServiceName    string // overridden by cfg.ServiceName when set
	ServiceVersion string // falls back to runtime build info
	Environment    string // e.g. "homelab", "dev", "production"
}

// New builds a Provider from the given config. When cfg.Enabled is
// false, returns a no-op Provider immediately (no network, no
// goroutines). stdoutHandler is the existing slog handler to keep
// writing to stdout; it's combined with the OTel log bridge in
// SlogHandler().
func New(ctx context.Context, cfg *config.OTelConfig, info BuildInfo, stdoutHandler slog.Handler) (*Provider, error) {
	if cfg == nil || !cfg.Enabled {
		return newNoop(stdoutHandler), nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	res, err := buildResource(cfg, info)
	if err != nil {
		return nil, fmt.Errorf("building resource: %w", err)
	}

	p := &Provider{
		enabled:       true,
		stdoutHandler: stdoutHandler,
	}

	// Traces
	if cfg.Traces.Enabled {
		exporter, err := newTraceExporter(ctx, cfg)
		if err != nil {
			_ = p.Shutdown(context.Background())
			return nil, fmt.Errorf("trace exporter: %w", err)
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		p.tracerProvider = tp
		p.shutdownFns = append(p.shutdownFns, tp.Shutdown)
	} else {
		p.tracerProvider = tracenoop.NewTracerProvider()
	}

	// Metrics
	if cfg.Metrics.Enabled {
		exporter, err := newMetricExporter(ctx, cfg)
		if err != nil {
			_ = p.Shutdown(context.Background())
			return nil, fmt.Errorf("metric exporter: %w", err)
		}
		interval := cfg.Metrics.Interval
		if interval <= 0 {
			interval = 60 * time.Second
		}
		reader := sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(interval),
		)
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		p.meterProvider = mp
		p.shutdownFns = append(p.shutdownFns, mp.Shutdown)

		// Go runtime metrics (heap, goroutines, GC).
		if cfg.Metrics.Runtime {
			if err := otelruntime.Start(otelruntime.WithMeterProvider(mp)); err != nil {
				// Non-fatal: runtime metrics failing shouldn't crash the
				// service, but it should be loud in logs. The slog
				// handler isn't wired yet, so surface via the default.
				slog.Default().Warn("otel runtime metrics failed to start", "error", err)
			}
		}
	} else {
		p.meterProvider = metricnoop.NewMeterProvider()
	}

	// Logs
	if cfg.Logs.Enabled {
		exporter, err := newLogExporter(ctx, cfg)
		if err != nil {
			_ = p.Shutdown(context.Background())
			return nil, fmt.Errorf("log exporter: %w", err)
		}
		processor := sdklog.NewBatchProcessor(exporter)
		lp := sdklog.NewLoggerProvider(
			sdklog.WithProcessor(processor),
			sdklog.WithResource(res),
		)
		p.loggerProvider = lp
		p.shutdownFns = append(p.shutdownFns, lp.Shutdown)
	} else {
		p.loggerProvider = lognoop.NewLoggerProvider()
	}

	// Install global providers so libraries we don't control (paho,
	// fiber middleware when called without explicit options, etc.) pick
	// them up automatically.
	otel.SetTracerProvider(p.tracerProvider)
	otel.SetMeterProvider(p.meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return p, nil
}

// newNoop constructs a Provider whose accessors return OTel no-op
// implementations. stdoutHandler is preserved so SlogHandler() still
// returns the existing slog destination even when telemetry is off.
func newNoop(stdoutHandler slog.Handler) *Provider {
	return &Provider{
		tracerProvider: tracenoop.NewTracerProvider(),
		meterProvider:  metricnoop.NewMeterProvider(),
		loggerProvider: lognoop.NewLoggerProvider(),
		stdoutHandler:  stdoutHandler,
	}
}

// Enabled reports whether the provider was built from a live OTel
// configuration. Useful for "log that telemetry is active" one-shots.
func (p *Provider) Enabled() bool {
	return p.enabled
}

// TracerProvider returns the underlying OTel TracerProvider.
func (p *Provider) TracerProvider() trace.TracerProvider {
	return p.tracerProvider
}

// MeterProvider returns the underlying OTel MeterProvider.
func (p *Provider) MeterProvider() metric.MeterProvider {
	return p.meterProvider
}

// LoggerProvider returns the underlying OTel LoggerProvider.
func (p *Provider) LoggerProvider() log.LoggerProvider {
	return p.loggerProvider
}

// Tracer returns a named tracer. Use package paths as names
// (`internal/ble`, `internal/mqtt`, etc.) so traces are grouped
// sensibly in Tempo/Grafana.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.tracerProvider.Tracer(name)
}

// Meter returns a named meter for metric instruments.
func (p *Provider) Meter(name string) metric.Meter {
	return p.meterProvider.Meter(name)
}

// SlogHandler returns a slog.Handler that writes to the stdout handler
// and, when OTel is enabled, also mirrors every record into the OTel
// log bridge so it lands in Loki via the collector. Safe to call on
// disabled providers — returns just the stdout handler in that case.
func (p *Provider) SlogHandler() slog.Handler {
	if !p.enabled {
		return p.stdoutHandler
	}
	otelHandler := otelslog.NewHandler("coolledux-controller",
		otelslog.WithLoggerProvider(p.loggerProvider),
	)
	return &multiHandler{handlers: []slog.Handler{p.stdoutHandler, otelHandler}}
}

// Shutdown flushes and closes every registered provider. Safe to call
// multiple times; the second call is a no-op.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	var err error
	p.shutdownOnce.Do(func() {
		var errs []error
		// Reverse order so logs flush after anything that might still
		// be writing to them.
		for i := len(p.shutdownFns) - 1; i >= 0; i-- {
			if fnErr := p.shutdownFns[i](ctx); fnErr != nil {
				errs = append(errs, fnErr)
			}
		}
		err = errors.Join(errs...)
	})
	return err
}

// buildResource composes the OTel resource (service.name, version,
// environment, plus any user-supplied attrs).
func buildResource(cfg *config.OTelConfig, info BuildInfo) (*resource.Resource, error) {
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = info.ServiceName
	}
	if serviceName == "" {
		serviceName = "coolledux-controller"
	}
	serviceVersion := cfg.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = info.ServiceVersion
	}
	if serviceVersion == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			serviceVersion = bi.Main.Version
		}
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
	}
	if serviceVersion != "" && serviceVersion != "(devel)" {
		attrs = append(attrs, semconv.ServiceVersion(serviceVersion))
	}
	if info.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(info.Environment))
	}
	for k, v := range cfg.ResourceAttrs {
		attrs = append(attrs, attribute.String(k, v))
	}

	return resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, attrs...))
}
