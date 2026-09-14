package content

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// Los nombres y las firmas de estos metodos los dicta ServerInterface, que sale
// del contrato. Si se renombra una operacion en api/openapi.yaml y no aqui, la
// afirmacion de module.go deja de compilar.

func (m *Module) LeerPaginaPublica(w http.ResponseWriter, r *http.Request, slug Slug) {
	pagina, err := m.svc.Publica(r.Context(), slug)
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

func (m *Module) ListarPaginas(w http.ResponseWriter, r *http.Request, _ ListarPaginasParams) {
	pagina, err := m.svc.Pagina(r.Context(), r.URL.Query())
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

func (m *Module) CrearPagina(w http.ResponseWriter, r *http.Request) {
	var nueva PaginaNueva
	if prob := httpx.DecodeJSON(w, r, &nueva); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	creada, err := m.svc.Crear(r.Context(), nueva)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", httpx.ETag(creada.Version))
	httpx.Created(w, r, "/api/v1/pages/"+creada.Id.String(), creada)
}

func (m *Module) LeerPagina(w http.ResponseWriter, r *http.Request, id PaginaId) {
	pagina, err := m.svc.Leer(r.Context(), id)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", httpx.ETag(pagina.Version))
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

// GuardarPagina exige la version que el cliente creia estar editando. Que
// `If-Match` venga lo garantiza el codigo generado; su forma se comprueba aqui.
func (m *Module) GuardarPagina(w http.ResponseWriter, r *http.Request, id PaginaId, params GuardarPaginaParams) {
	version, prob := httpx.VersionFromIfMatch(params.IfMatch)
	if prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	var mod PaginaModificacion
	if prob := httpx.DecodeJSON(w, r, &mod); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	guardada, err := m.svc.Guardar(r.Context(), id, version, mod)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", httpx.ETag(guardada.Version))
	httpx.WriteJSON(w, r, http.StatusOK, guardada)
}

func (m *Module) BorrarPagina(w http.ResponseWriter, r *http.Request, id PaginaId) {
	if err := m.svc.Borrar(r.Context(), id); err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (m *Module) ListarVersiones(w http.ResponseWriter, r *http.Request, id PaginaId, _ ListarVersionesParams) {
	pagina, err := m.svc.Versiones(r.Context(), id, r.URL.Query())
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

// PublicarPagina no manda ETag. Publicar no cambia la version del borrador, y
// devolver la misma cabecera invitaria a pensar que si.
func (m *Module) PublicarPagina(w http.ResponseWriter, r *http.Request, id PaginaId) {
	var pub Publicacion
	if prob := httpx.DecodeJSON(w, r, &pub); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	publicada, err := m.svc.Publicar(r.Context(), id, pub)
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, publicada)
}

// fallo es la unica salida de error de los handlers del modulo. Un Problem ya
// sabe que contestar; lo demas es un fallo nuestro y sale como 500 generico.
func (m *Module) fallo(w http.ResponseWriter, r *http.Request, err error) {
	var prob *httpx.Problem
	if errors.As(err, &prob) {
		httpx.WriteProblem(w, r, prob)
		return
	}

	slog.ErrorContext(r.Context(), "content: fallo la operacion",
		slog.String("error", err.Error()),
		slog.String("traceId", observ.TraceIDFrom(r.Context())))
	httpx.WriteProblem(w, r, httpx.Internal())
}
