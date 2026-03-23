package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OTelMiddleware wraps requests with otelhttp instrumentation.
func OTelMiddleware(service string) func(http.Handler) http.Handler {
	return otelhttp.NewMiddleware(service)
}
