package content

import (
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
// Routes(). Montar LeerPagina en una ruta sin `{id}` compila igual y falla en
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
		peticion{metodo: http.MethodGet, ruta: "/api/v1/pages?size=abc"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q; lo generado responde text/plain si no se le cambia el manejador", ct)
	}
	if p := problema(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "size" {
		t.Errorf("errors = %+v; tiene que nombrar el parametro culpable", p.Errors)
	}
}

// La forma del Problem generado contra httpx.Problem ya la compara
// settings/contrato_test.go: sale del mismo esquema del contrato, asi que una
// segunda copia de esa prueba aqui no detectaria nada nuevo.
