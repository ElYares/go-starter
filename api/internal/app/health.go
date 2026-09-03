package app

import (
	"context"
	"net/http"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// Los nombres y las firmas los dicta ServerInterface, que sale del contrato.
// Ver la afirmacion en app.go.

// Healthz responde si el proceso vive. No toca la base a proposito: mezclar las
// dos preguntas hace que un reinicio de la base parezca un proceso muerto y
// dispare reinicios que no arreglan nada.
func (a *App) Healthz(w http.ResponseWriter, r *http.Request) {
	// El cuerpo sale de los tipos del contrato, no de un map escrito a mano. Un
	// map acepta cualquier clave y cualquier valor, asi que una respuesta que se
	// aparta del contrato compila igual y nadie se entera.
	httpx.WriteJSON(w, r, http.StatusOK, Health{Status: HealthStatusOk})
}

// Readyz responde si el proceso puede atender: hoy, si la base contesta.
func (a *App) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.pool.Ping(ctx); err != nil {
		trace := observ.TraceIDFrom(ctx)
		a.log.WarnContext(ctx, "readyz: la base no responde",
			"error", err, "traceId", trace)

		httpx.WriteJSON(w, r, http.StatusServiceUnavailable, Readiness{
			Status:  ReadinessStatusUnavailable,
			Checks:  map[string]ReadinessChecks{"db": Down},
			TraceId: &trace,
		})
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, Readiness{
		Status: ReadinessStatusOk,
		Checks: map[string]ReadinessChecks{"db": Up},
	})
}
