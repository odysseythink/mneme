package daemon

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// Transport identifies which listener served a request.
type Transport int

const (
	TransportUnix Transport = iota
	TransportTCP
)

type ctxKey int

const (
	ctxTransport ctxKey = iota
)

func WithTransport(ctx context.Context, t Transport) context.Context {
	return context.WithValue(ctx, ctxTransport, t)
}

func transportFromCtx(ctx context.Context) Transport {
	v, _ := ctx.Value(ctxTransport).(Transport)
	return v
}

func RecoverMW(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("http", fmt.Sprintf("panic: %v\n%s", rec, debug.Stack()))
					http.Error(w, `{"error":"internal","code":"internal"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMW gates TCP requests with a bearer token, cookie, or bootstrap query param.
// Unix socket requests bypass authentication entirely.
//
// On TCP, accepts in priority order:
//  1. `Authorization: Bearer <token>` header
//  2. `mneme_token` cookie (M10a)
//  3. `?token=<token>` query param on GET / (M10a, bootstrap-only)
func AuthMW(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if transportFromCtx(r.Context()) == TransportUnix {
				next.ServeHTTP(w, r)
				return
			}
			// 1. Bearer header
			h := r.Header.Get("Authorization")
			if strings.HasPrefix(h, "Bearer ") {
				given := strings.TrimPrefix(h, "Bearer ")
				if subtle.ConstantTimeCompare([]byte(given), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			// 2. Cookie (M10a)
			if c, err := r.Cookie("mneme_token"); err == nil {
				if subtle.ConstantTimeCompare([]byte(c.Value), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			// 3. Bootstrap-only ?token= on GET /
			if r.Method == http.MethodGet && r.URL.Path == "/" {
				if q := r.URL.Query().Get("token"); q != "" {
					if subtle.ConstantTimeCompare([]byte(q), []byte(token)) == 1 {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w, `{"error":"unauthorized","code":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
}

func BodyLimitMW(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func LogMW(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &recordingResponseWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			dur := time.Since(start)
			log.Info("http", fmt.Sprintf("method=%s path=%s status=%d dur_ms=%d", r.Method, r.URL.Path, rw.status, dur.Milliseconds()))
		})
	}
}

type recordingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (r *recordingResponseWriter) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
