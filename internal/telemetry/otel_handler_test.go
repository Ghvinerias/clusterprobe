package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type captureHandler struct {
	enabled bool
	err     error
	records []slog.Record
	attrs   []slog.Attr
	groups  []string
}

func (h *captureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.enabled
}

func (h *captureHandler) Handle(ctx context.Context, record slog.Record) error {
	h.records = append(h.records, record)
	if h.err != nil {
		return h.err
	}
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	next := *h
	next.groups = append(append([]string{}, h.groups...), name)
	return &next
}

func TestFanoutHandlerEnabled(t *testing.T) {
	handler := &fanoutHandler{
		json: &captureHandler{enabled: false},
		otel: &captureHandler{enabled: true},
	}

	require.True(t, handler.Enabled(context.Background(), slog.LevelInfo))

	handler = &fanoutHandler{
		json: &captureHandler{enabled: false},
		otel: &captureHandler{enabled: false},
	}
	require.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
}

func TestFanoutHandlerHandleInjectsTraceID(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "span")
	defer span.End()

	jsonHandler := &captureHandler{enabled: true}
	otelHandler := &captureHandler{enabled: true}
	handler := &fanoutHandler{json: jsonHandler, otel: otelHandler}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	require.NoError(t, handler.Handle(ctx, record))

	require.Len(t, jsonHandler.records, 1)
	require.True(t, recordHasAttr(jsonHandler.records[0], "trace_id"))
}

func TestFanoutHandlerHandleErrors(t *testing.T) {
	jsonHandler := &captureHandler{enabled: true, err: errors.New("json fail")}
	otelHandler := &captureHandler{enabled: true}
	handler := &fanoutHandler{json: jsonHandler, otel: otelHandler}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	require.Error(t, handler.Handle(context.Background(), record))

	jsonHandler = &captureHandler{enabled: true}
	otelHandler = &captureHandler{enabled: true, err: errors.New("otel fail")}
	handler = &fanoutHandler{json: jsonHandler, otel: otelHandler}

	require.Error(t, handler.Handle(context.Background(), record))
}

func TestFanoutHandlerWithAttrsAndGroup(t *testing.T) {
	jsonHandler := &captureHandler{enabled: true}
	otelHandler := &captureHandler{enabled: true}
	handler := &fanoutHandler{json: jsonHandler, otel: otelHandler}

	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("k", "v")})
	fanout, ok := withAttrs.(*fanoutHandler)
	require.True(t, ok)

	jsonWithAttrs, ok := fanout.json.(*captureHandler)
	require.True(t, ok)
	require.Len(t, jsonWithAttrs.attrs, 1)

	withGroup := handler.WithGroup("group")
	fanout, ok = withGroup.(*fanoutHandler)
	require.True(t, ok)

	jsonWithGroup, ok := fanout.json.(*captureHandler)
	require.True(t, ok)
	require.Len(t, jsonWithGroup.groups, 1)
}

func recordHasAttr(record slog.Record, key string) bool {
	found := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			found = true
			return false
		}
		return true
	})
	return found
}
