package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging logs each HTTP request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		latency := time.Since(start)
		traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		method := sanitizeLogValue(r.Method)
		path := sanitizeLogValue(r.URL.Path)

		// #nosec G706 -- log values are sanitized to avoid line breaks.
		slog.Info(
			"http_request",
			"method", method,
			"path", path,
			"status", wrapped.status,
			"latency_ms", latency.Milliseconds(),
			"trace_id", traceID,
		)
	})
}

func sanitizeLogValue(value string) string {
	replacer := strings.NewReplacer("\n", " ", "\r", " ")
	return replacer.Replace(value)
}
