package media

import (
	"net/http"
	"slices"
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
