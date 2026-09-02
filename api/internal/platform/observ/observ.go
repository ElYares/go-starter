// Package observ es lo que hace que "no me deja guardar" se convierta en un log
// encontrable: un identificador por peticion que viaja en la respuesta y en
// cada linea de log.
package observ

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type ctxKey struct{}

// HeaderTraceID es el nombre de la cabecera de ida y de vuelta. Si el cliente
// ya trae una, se propaga en vez de generar otra: asi una peticion que cruza
// dos servicios se sigue con un solo identificador.
const HeaderTraceID = "X-Trace-Id"

func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

func newTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Un traceId degradado es preferible a tumbar la peticion: sirve para
		// correlacionar, no para autenticar.
		return "trace-unavailable"
	}
	return hex.EncodeToString(b[:])
}

// TraceID va primero en la cadena, porque todo lo que sigue lo registra.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderTraceID)
		if id == "" {
			id = newTraceID()
		}
		w.Header().Set(HeaderTraceID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

// Recover va antes del logger, para que un panico tambien quede registrado por
// el. La respuesta es generica a proposito: el detalle se queda en el log.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panico en handler",
						slog.Any("panic", rec),
						slog.String("traceId", TraceIDFrom(r.Context())),
						slog.String("path", r.URL.Path),
					)
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"title":"Error interno","status":500,"code":"INTERNAL_ERROR","traceId":"` +
						TraceIDFrom(r.Context()) + `"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.InfoContext(r.Context(), "peticion",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("took", time.Since(start)),
				slog.String("traceId", TraceIDFrom(r.Context())),
			)
		})
	}
}

// Chain aplica los middleware en el orden en que se leen: Chain(h, a, b) corre
// a, luego b, luego h.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
