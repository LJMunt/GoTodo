package logging

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// RequestLogger provides a chi middleware that logs HTTP requests with zerolog
func RequestLogger(base zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			reqID := middleware.GetReqID(r.Context())
			l := base.With().
				Str("request_id", reqID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_ip", r.RemoteAddr).
				Logger()

			// attach request-scoped logger to context
			ctx := l.WithContext(r.Context())
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			event := l.Info()
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/health" {
				event = l.Debug()
			}

			event.
				Int("status", ww.Status()).
				Dur("duration", time.Since(start)).
				Int("bytes", ww.BytesWritten()).
				Msg("http_request")
		})
	}
}
