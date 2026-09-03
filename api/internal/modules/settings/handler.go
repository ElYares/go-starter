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

// Los nombres y las firmas de estos metodos NO son una eleccion: los dicta
// ServerInterface, que sale del contrato. Si se renombra una operacion en
// api/openapi.yaml y no aqui, la afirmacion de module.go deja de compilar.

// ListarSettingsPublicas devuelve un objeto plano de clave a valor, no el
// envoltorio de coleccion.
//
// Es la excepcion escrita del molde, no un olvido: la landing consume la
// configuracion ENTERA en cada carga renderizada en servidor, y darle paginas
// la obligaria a pedir varias para dibujar una sola pantalla. El envoltorio es
// para colecciones que crecen; esta no crece.
func (m *Module) ListarSettingsPublicas(w http.ResponseWriter, r *http.Request) {
	m.mapa(w, r, m.svc.Publicas)
}

func (m *Module) mapa(w http.ResponseWriter, r *http.Request, fn func(context.Context) ([]Setting, error)) {
	items, err := fn(r.Context())
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	valores := make(SettingsMap, len(items))
	for _, s := range items {
		valores[s.Key] = s.Value
	}

	httpx.WriteJSON(w, r, http.StatusOK, valores)
}

// ListarSettings es el endpoint de coleccion. Lo consume el dashboard.
//
// `params` llega ya enlazado por el codigo generado, que comprueba los TIPOS
// que declara el contrato: `?size=abc` no llega hasta aqui. `paging` hace lo
// otro, que el contrato no puede expresar: la lista blanca de campos, el tope
// efectivo y la traduccion a columnas. Son comprobaciones distintas, y por eso
// conviven; paging es la mas estricta de las dos, asi que nunca se contradicen.
func (m *Module) ListarSettings(w http.ResponseWriter, r *http.Request, _ ListarSettingsParams) {
	pagina, err := m.svc.Pagina(r.Context(), r.URL.Query())
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, pagina)
}

func (m *Module) LeerSetting(w http.ResponseWriter, r *http.Request, key string) {
	item, err := m.svc.Leer(r.Context(), key)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", etag(item.Version))
	httpx.WriteJSON(w, r, http.StatusOK, item)
}

func (m *Module) CrearSetting(w http.ResponseWriter, r *http.Request) {
	// El cuerpo se decodifica con httpx y no con lo generado: la decodificacion
	// estricta —un campo desconocido es 400, no un silencio— es una promesa de
	// la plataforma, y el contrato la declara con `additionalProperties: false`.
	var nueva SettingNuevo
	if prob := httpx.DecodeJSON(w, r, &nueva); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	creada, err := m.svc.Crear(r.Context(), nueva)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	w.Header().Set("ETag", etag(creada.Version))
	httpx.Created(w, r, "/api/v1/settings/"+creada.Key, creada)
}

// ReemplazarSetting exige la version que el cliente creia estar editando. Que
// `If-Match` sea obligatoria la garantiza el contrato: el codigo generado
// responde antes de llegar aqui si falta. Lo que queda por comprobar es la
// FORMA del valor, que el contrato no puede expresar.
func (m *Module) ReemplazarSetting(w http.ResponseWriter, r *http.Request, key string, params ReemplazarSettingParams) {
	version, prob := versionDeETag(params.IfMatch)
	if prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	var mod SettingModificacion
	if prob := httpx.DecodeJSON(w, r, &mod); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	actualizada, err := m.svc.Reemplazar(r.Context(), key, version, mod)
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

func versionDeETag(crudo string) (int, *httpx.Problem) {
	crudo = strings.TrimSpace(crudo)

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
