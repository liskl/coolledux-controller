package telemetry

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/liskl/coolledux-controller/internal/config"
)

func TestNew_DisabledReturnsNoop(t *testing.T) {
	ctx := context.Background()
	p, err := New(ctx, &config.OTelConfig{Enabled: false}, BuildInfo{}, slog.NewJSONHandler(io.Discard, nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Enabled() {
		t.Error("Enabled() = true for disabled config")
	}
	// Spot-check that the providers are the OTel noops. The concrete
	// types here are from go.opentelemetry.io/otel/{trace,metric,log}/noop.
	if _, ok := p.TracerProvider().(tracenoop.TracerProvider); !ok {
		// tracenoop.NewTracerProvider returns a value type; accept both
		// forms by checking the package name.
		if !strings.Contains(strings.ToLower(providerType(p.TracerProvider())), "noop") {
			t.Errorf("expected noop tracer provider, got %T", p.TracerProvider())
		}
	}
	if _, ok := p.MeterProvider().(metricnoop.MeterProvider); !ok {
		if !strings.Contains(strings.ToLower(providerType(p.MeterProvider())), "noop") {
			t.Errorf("expected noop meter provider, got %T", p.MeterProvider())
		}
	}
	var _ log.LoggerProvider = lognoop.NewLoggerProvider()
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown on noop: %v", err)
	}
}

func TestNew_NilConfigReturnsNoop(t *testing.T) {
	p, err := New(context.Background(), nil, BuildInfo{}, slog.NewJSONHandler(io.Discard, nil))
	if err != nil {
		t.Fatalf("New(nil cfg): %v", err)
	}
	if p.Enabled() {
		t.Error("nil cfg should produce disabled provider")
	}
}

func TestNew_RequiresEndpointWhenEnabled(t *testing.T) {
	_, err := New(context.Background(), &config.OTelConfig{Enabled: true}, BuildInfo{}, nil)
	if err == nil {
		t.Fatal("expected error when enabled with no endpoint")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("error should mention endpoint, got %q", err)
	}
}

func TestNew_UnknownProtocolRejected(t *testing.T) {
	cfg := &config.OTelConfig{
		Enabled:  true,
		Endpoint: "localhost:4317",
		Protocol: "carrier-pigeon",
	}
	_, err := New(context.Background(), cfg, BuildInfo{}, nil)
	if err == nil {
		t.Fatal("expected error for invalid protocol")
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	p := newNoop(slog.NewJSONHandler(io.Discard, nil))

	// Register a counting shutdown fn so we can confirm it runs exactly once.
	var calls atomic.Int32
	p.enabled = true
	p.shutdownFns = append(p.shutdownFns, func(context.Context) error {
		calls.Add(1)
		return nil
	})

	ctx := context.Background()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("shutdown fn ran %d times, want 1", got)
	}
}

// recordingHandler captures every slog.Record it receives so the
// multiHandler fanout can be asserted without needing a real OTLP server.
type recordingHandler struct {
	level   slog.Level
	records []slog.Record
}

func (r *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (r *recordingHandler) Handle(_ context.Context, rec slog.Record) error {
	// slog.Record docs: "Handlers that store records must call Clone first."
	// The production multiHandler already clones; this test handler should
	// match so it's robust against future slog runtime changes that pool
	// internal attr storage.
	r.records = append(r.records, rec.Clone())
	return nil
}
func (r *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return r }
func (r *recordingHandler) WithGroup(name string) slog.Handler       { return r }

func TestMultiHandler_FansOut(t *testing.T) {
	a := &recordingHandler{}
	b := &recordingHandler{}
	h := NewMultiHandler(a, b)

	logger := slog.New(h)
	logger.Info("hello", "k", "v")

	if got := len(a.records); got != 1 {
		t.Errorf("handler A got %d records, want 1", got)
	}
	if got := len(b.records); got != 1 {
		t.Errorf("handler B got %d records, want 1", got)
	}
}

func TestSlogHandler_DisabledReturnsStdoutOnly(t *testing.T) {
	stdout := &recordingHandler{}
	p := newNoop(stdout)
	if p.SlogHandler() != stdout {
		t.Error("disabled provider should return the stdout handler unchanged")
	}
}

// providerType returns the Go type name of v as a string. Used to avoid
// importing the OTel noop package paths when checking provider kind.
func providerType(v any) string {
	return slog.AnyValue(v).String()
}
