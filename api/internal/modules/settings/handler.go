package settings

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

func (m *Module) listarPublicas(w http.ResponseWriter, r *http.Request) {
	m.responder(w, r, m.svc.Publicas)
}

func (m *Module) listarTodas(w http.ResponseWriter, r *http.Request) {
	m.responder(w, r, m.svc.Todas)
}

// El envoltorio de coleccion con paginacion llega en HU-004. Aqui todavia sale
// un objeto plano, y se dice: la configuracion se consume entera en cada carga
// de la landing, asi que paginarla seria inventar un problema.
func (m *Module) responder(w http.ResponseWriter, r *http.Request, fn func(context.Context) ([]Setting, error)) {
	items, err := fn(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "settings: fallo la consulta",
			slog.String("error", err.Error()),
			slog.String("traceId", observ.TraceIDFrom(r.Context())))
		httpx.WriteProblem(w, r, httpx.Internal())
		return
	}

	valores := make(map[string]any, len(items))
	for _, s := range items {
		valores[s.Key] = s.Value
	}

	httpx.WriteJSON(w, r, http.StatusOK, valores)
}
