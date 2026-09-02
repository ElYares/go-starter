package app

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// healthz responde si el proceso vive. No toca la base a proposito: mezclar las
// dos preguntas hace que un reinicio de la base parezca un proceso muerto y
// dispare reinicios que no arreglan nada.
func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz responde si el proceso puede atender: hoy, si la base contesta.
func (a *App) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.pool.Ping(ctx); err != nil {
		a.log.WarnContext(ctx, "readyz: la base no responde",
			"error", err, "traceId", observ.TraceIDFrom(ctx))
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "unavailable",
			"checks":  map[string]string{"db": "down"},
			"traceId": observ.TraceIDFrom(ctx),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"checks": map[string]string{"db": "up"},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
