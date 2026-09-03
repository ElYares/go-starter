package settings

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// muxEspia no enruta nada: solo anota los patrones que le pasan.
//
// Sirve para preguntarle al codigo generado que rutas describe el contrato para
// este modulo, sin leer el YAML ni depender de un parser. Es la unica forma de
// comparar el contrato con lo que Routes() monta a mano.
type muxEspia struct{ patrones []string }

func (m *muxEspia) HandleFunc(patron string, _ func(http.ResponseWriter, *http.Request)) {
	m.patrones = append(m.patrones, patron)
}

func (m *muxEspia) ServeHTTP(http.ResponseWriter, *http.Request) {}

// El agujero que esta prueba tapa: la interfaz generada garantiza que los
// HANDLERS cuadren con el contrato, pero los PATRONES se escriben a mano en
// Routes(). Montar LeerSetting en una ruta sin `{key}` compila igual y falla en
// la primera peticion.
func TestLasRutasMontadasSonExactamenteLasDelContrato(t *testing.T) {
	espia := &muxEspia{}
	HandlerFromMuxWithBaseURL(&Module{}, espia, "/api/v1")

	r := httpx.NewRouter()
	(&Module{}).Routes(r)

	montadas := make([]string, 0)
	for _, rt := range r.Routes() {
		montadas = append(montadas, rt.Method+" "+rt.Pattern)
	}

	slices.Sort(montadas)
	slices.Sort(espia.patrones)

	if !slices.Equal(montadas, espia.patrones) {
		t.Errorf("las rutas montadas no son las del contrato\n  contrato: %v\n  montadas: %v",
			espia.patrones, montadas)
	}
}

// El manejador de errores por omision de lo generado responde con http.Error:
// texto plano y sin traceId. Es el mismo agujero que el 404 de omision de
// net/http, y se ve igual de bien en el navegador mientras rompe a los clientes.
func TestUnParametroMalTipadoSaleComoProblemJson(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(),
		peticion{metodo: http.MethodGet, ruta: "/api/v1/settings?size=abc"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q; lo generado responde text/plain si no se le cambia el manejador", ct)
	}

	p := problema(t, rec)
	if p.Code != httpx.CodeValidationFailed {
		t.Errorf("code = %q", p.Code)
	}
	if len(p.Errors) != 1 || p.Errors[0].Field != "size" {
		t.Errorf("errors = %+v; tiene que nombrar el parametro culpable", p.Errors)
	}
}

// Y la cabecera obligatoria que ahora exige el contrato, no el codigo.
func TestLaCabeceraObligatoriaLaExigeElContrato(t *testing.T) {
	rec := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand", cuerpo: `{"value":1}`,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	p := problema(t, rec)
	if len(p.Errors) != 1 || p.Errors[0].Field != "If-Match" {
		t.Errorf("errors = %+v; tiene que nombrar la cabecera que falta", p.Errors)
	}
}

// `Problem` se genera en este paquete porque lo referencian las respuestas de
// error del modulo, pero la API contesta con httpx.Problem, escrito a mano.
//
// Los dos describen la misma cosa, asi que pueden separarse sin que nada falle:
// el contrato diria una forma y la API devolveria otra. Esta prueba compara las
// dos por su JSON, que es lo unico que ve el cliente.
func TestElProblemDelContratoYElDeLaApiTienenLaMismaForma(t *testing.T) {
	detalle, instancia, tipo := "d", "i", "about:blank"

	delContrato := Problem{
		Code: "X", Detail: &detalle, Instance: &instancia, Status: 400,
		Title: "t", TraceId: "tr", Type: &tipo,
		Errors: &[]struct {
			Code    string `json:"code"`
			Field   string `json:"field"`
			Message string `json:"message"`
		}{{Code: "c", Field: "f", Message: "m"}},
	}

	deLaApi := httpx.Problem{
		Type: tipo, Title: "t", Status: 400, Detail: detalle, Instance: instancia,
		Code: "X", TraceID: "tr",
		Errors: []httpx.FieldIssue{{Field: "f", Code: "c", Message: "m"}},
	}

	if a, b := clavesJSON(t, delContrato), clavesJSON(t, deLaApi); !slices.Equal(a, b) {
		t.Errorf("las claves no coinciden\n  contrato: %v\n  api:      %v", a, b)
	}

	// Y las de dentro de errors[], que es donde el cliente resalta el campo.
	var contrato, api map[string]any
	crudoC, _ := json.Marshal(delContrato)
	crudoA, _ := json.Marshal(deLaApi)
	_ = json.Unmarshal(crudoC, &contrato)
	_ = json.Unmarshal(crudoA, &api)

	if !slices.Equal(clavesDelPrimerError(contrato), clavesDelPrimerError(api)) {
		t.Errorf("errors[] difiere: contrato %v, api %v",
			clavesDelPrimerError(contrato), clavesDelPrimerError(api))
	}
}

func clavesJSON(t *testing.T, v any) []string {
	t.Helper()
	crudo, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(crudo, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	slices.Sort(claves)
	return claves
}

func clavesDelPrimerError(m map[string]any) []string {
	lista, ok := m["errors"].([]any)
	if !ok || len(lista) == 0 {
		return nil
	}
	primero, ok := lista[0].(map[string]any)
	if !ok {
		return nil
	}
	claves := make([]string, 0, len(primero))
	for k := range primero {
		claves = append(claves, k)
	}
	slices.Sort(claves)
	return claves
}
