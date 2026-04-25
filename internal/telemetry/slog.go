package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

// multiHandler fans every slog.Record to each of its underlying handlers.
// Used to keep writing to stdout *and* forwarding records through the
// OTel log bridge to the collector when OTel is enabled.
//
// Enabled() checks are applied per underlying handler, so the OTel
// handler's level filter (e.g. drop Debug records from the log pipeline
// while keeping them on stdout) works independently.
type multiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler is exported for tests and any caller that wants to
// compose multiple slog handlers. The regular service path uses
// Provider.SlogHandler() which returns a *multiHandler when enabled.
//
// nil entries in the variadic are filtered out (consistent with the
// telemetry package's "forgiving > runtime panic" stance — see the
// nil-stdoutHandler default in telemetry.go's ensureSlogHandler).
// After filtering: zero handlers → discard handler; one handler →
// returned directly without the multi-handler wrapper; otherwise the
// surviving handlers are wrapped.
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	switch len(filtered) {
	case 0:
		return slog.NewJSONHandler(io.Discard, nil)
	case 1:
		return filtered[0]
	default:
		return &multiHandler{handlers: filtered}
	}
}

// Enabled reports whether any underlying handler wants the record.
func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle fans the record out; errors from each handler are joined so
// one failing destination doesn't swallow the others.
func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, record.Level) {
			continue
		}
		// slog.Record.Clone is required because handlers are allowed to
		// mutate the record (add attributes, etc.).
		if err := h.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a handler that adds attrs to every record, applied
// uniformly to each destination.
func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

// WithGroup returns a handler that groups subsequent attrs under name.
func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}
