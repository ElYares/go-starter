package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// Recover convierte un panico en un 500 con la misma forma que cualquier otro
// error de la API, y deja el stack completo en el log junto al traceId.
//
// Vive aqui y no en observ porque quien decide la FORMA de la respuesta es
// httpx; observ solo sabe de logs e identificadores. Va antes del logger en la
// cadena, para que la peticion que revento tambien quede registrada.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// El cliente se desconecto a media respuesta: no es un fallo
				// nuestro y no hay a quien contestarle.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panico en handler",
					slog.Any("panic", rec),
					slog.String("traceId", observ.TraceIDFrom(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				WriteProblem(w, r, Internal())
			}()
			next.ServeHTTP(w, r)
		})
	}
}
