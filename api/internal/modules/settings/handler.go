package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// listarPublicas devuelve un objeto plano de clave a valor, no el envoltorio de
// coleccion.
//
// Es la excepcion escrita del molde, no un olvido: la landing consume la
// configuracion ENTERA en cada carga renderizada en servidor, y darle paginas
// la obligaria a pedir varias para dibujar una sola pantalla. El envoltorio es
// para colecciones que crecen; esta no crece.
func (m *Module) listarPublicas(w http.ResponseWriter, r *http.Request) {
	m.mapa(w, r, m.svc.Publicas)
}

func (m *Module) mapa(w http.ResponseWriter, r *http.Request, fn func(context.Context) ([]Setting, error)) {
	items, err := fn(r.Context())
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	valores := make(map[string]any, len(items))
	for _, s := range items {
		valores[s.Key] = s.Value
	}

	httpx.WriteJSON(w, r, http.StatusOK, valores)
}

// listar es el endpoint de coleccion: envoltorio fijo, tope duro de size y
// listas blancas de orden y filtro. Lo consume el dashboard.
func (m *Module) listar(w http.ResponseWriter, r *http.Request) {
	pagina, err := m.svc.Pagina(r.Context(), r.URL.Query())
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

func (m *Module) leer(w http.ResponseWriter, r *http.Request) {
	item, err := m.svc.Leer(r.Context(), r.PathValue("key"))
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", etag(item.Version))
	httpx.WriteJSON(w, r, http.StatusOK, item)
}

func (m *Module) crear(w http.ResponseWriter, r *http.Request) {
	var e entrada
	if prob := httpx.DecodeJSON(w, r, &e); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	creada, err := m.svc.Crear(r.Context(), e)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", etag(creada.Version))
	httpx.Created(w, r, "/api/v1/settings/"+creada.Key, creada)
}

func (m *Module) reemplazar(w http.ResponseWriter, r *http.Request) {
	version, prob := versionPedida(r)
	if prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	var mod modificacion
	if prob := httpx.DecodeJSON(w, r, &mod); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	actualizada, err := m.svc.Reemplazar(r.Context(), r.PathValue("key"), version, mod)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", etag(actualizada.Version))
	httpx.WriteJSON(w, r, http.StatusOK, actualizada)
}

// etag es la version entre comillas. Es lo que el cliente devuelve en If-Match
// sin tener que entender que por dentro es un contador.
func etag(version int) string { return fmt.Sprintf("%q", strconv.Itoa(version)) }

// versionPedida lee If-Match. La cabecera es OBLIGATORIA en las escrituras:
// permitirla ausente convertiria cada guardado descuidado en una sobrescritura
// silenciosa, que es justo lo que el `version` existe para impedir.
//
// Va como 400 VALIDATION_FAILED y no como 428: la tabla de codigos de
// docs/04-reglas-de-crud.md seccion 2 es normativa y no tiene 428, y una
// cabecera obligatoria que falta es un parametro invalido.
func versionPedida(r *http.Request) (int, *httpx.Problem) {
	crudo := strings.TrimSpace(r.Header.Get("If-Match"))

	if crudo == "" {
		return 0, httpx.BadRequest("Falta la cabecera If-Match", httpx.FieldIssue{
			Field: "If-Match", Code: "required",
			Message: "Manda el ETag que devolvio la lectura, para no pisar el cambio de otra persona",
		})
	}

	// `*` significa "cualquiera": es una sobrescritura incondicional, que es
	// exactamente lo que aqui no se quiere permitir.
	if crudo == "*" {
		return 0, httpx.BadRequest("If-Match no acepta *", httpx.FieldIssue{
			Field: "If-Match", Code: "format",
			Message: "Manda el ETag concreto que devolvio la lectura",
		})
	}

	version, err := strconv.Atoi(strings.Trim(strings.TrimPrefix(crudo, "W/"), `"`))
	if err != nil || version < 1 {
		return 0, httpx.BadRequest("If-Match no tiene la forma de un ETag de este recurso",
			httpx.FieldIssue{
				Field: "If-Match", Code: "format",
				Message: `Se esperaba el ETag devuelto por la lectura, por ejemplo "7"`,
			})
	}

	return version, nil
}

// fallo es la unica salida de error de los handlers del modulo.
//
// Un *httpx.Problem que subio desde el service ya sabe que contestar. Lo que no
// es un Problem es un fallo nuestro: sale como 500 generico y el detalle se
// queda en el log, junto al mismo traceId que ve el cliente.
func (m *Module) fallo(w http.ResponseWriter, r *http.Request, err error) {
	var prob *httpx.Problem
	if errors.As(err, &prob) {
		httpx.WriteProblem(w, r, prob)
		return
	}

	slog.ErrorContext(r.Context(), "settings: fallo la operacion",
		slog.String("error", err.Error()),
		slog.String("traceId", observ.TraceIDFrom(r.Context())))
	httpx.WriteProblem(w, r, httpx.Internal())
}
