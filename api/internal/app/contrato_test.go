package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// rutasDeTodo monta plataforma y modulos, que es lo unico que da la foto
// completa de la superficie HTTP.
func rutasDeTodo(t *testing.T) []httpx.Route {
	t.Helper()

	// Se monta el registro de verdad y no una lista escrita a mano: asi el dia
	// que se agregue un modulo, estas pruebas lo miran sin que nadie tenga que
	// acordarse de agregarlo aqui.
	return appDePrueba(t, modulosDePrueba(t)...).router.Routes()
}

// El versionado no es opcional: un starter cuyos forks salen a produccion no
// puede romper clientes que no controla. Una ruta fuera de /api/v1 no tiene
// como versionarse despues.
func TestTodaLaSuperficieVivaBajoApiV1(t *testing.T) {
	rutas := rutasDeTodo(t)
	if len(rutas) == 0 {
		t.Fatal("no se monto ninguna ruta: la prueba pasaria por vacuidad")
	}

	for _, rt := range rutas {
		if !strings.HasPrefix(rt.Pattern, "/api/v1/") && rt.Pattern != "/api/v1" {
			t.Errorf("%s %s esta fuera de /api/v1", rt.Method, rt.Pattern)
		}
	}
}

// Lo publico vive bajo su propio prefijo a proposito: asi una ruta abierta al
// mundo es una linea que se lee en el diff, y no un `Require` que alguien
// olvido poner.
//
// Las excepciones se nombran una por una. Que la lista este escrita aqui es el
// punto: agregar una ruta publica fuera de /public obliga a tocar esta prueba,
// y eso se ve en la revision.
func TestLoPublicoViveBajoSuPrefijoSalvoLasExcepcionesEscritas(t *testing.T) {
	// Salud: la consultan el orquestador y el compose, que no tienen sesion. El
	// contrato las declara con `security: []`.
	exentas := map[string]bool{
		"GET /api/v1/healthz": true,
		"GET /api/v1/readyz":  true,
		// El login no puede exigir sesion: quien lo llama todavia no tiene
		// ninguna. Lo que si lo protege es el CSRF de la cadena global, que le
		// exige X-XSRF-TOKEN como a cualquier mutacion.
		"POST /api/v1/auth/login": true,
		// `me` SI exige sesion, con rbac.RequireSession, pero no un permiso con
		// nombre: todo el mundo puede leer su propio perfil. Como el guard no
		// anota ningun permiso en la ruta, desde aqui se ve igual que una ruta
		// sin proteger, y por eso tiene que estar escrita.
		"GET /api/v1/auth/me": true,
		// Refresh y logout tampoco exigen sesion: se llaman con el `at` ya
		// caducado. Los autentica el `rt` de su cookie, y el CSRF de la cadena
		// global les exige la cabecera como a toda mutacion.
		"POST /api/v1/auth/refresh": true,
		"POST /api/v1/auth/logout":  true,
		// La UI de exploracion del contrato. Solo existe con API_DOCS_ENABLED y
		// no es superficie publica de la API, por eso tampoco esta en el spec.
		"GET /api/v1/docs":         true,
		"GET /api/v1/openapi.yaml": true,
	}

	for _, rt := range rutasDeTodo(t) {
		if rt.Permission != "" {
			continue
		}

		clave := rt.Method + " " + rt.Pattern
		if exentas[clave] {
			continue
		}
		if !strings.HasPrefix(rt.Pattern, "/api/v1/public/") {
			t.Errorf("%s no exige permiso y no vive bajo /api/v1/public: o es un olvido, o falta escribirla como excepcion", clave)
		}
	}
}

// Y al reves: nada bajo /public puede exigir sesion. Una ruta publica con guard
// responde 401 a todo el mundo, y eso en la landing se ve como una pagina rota.
func TestNadaBajoPublicExigePermiso(t *testing.T) {
	for _, rt := range rutasDeTodo(t) {
		if strings.HasPrefix(rt.Pattern, "/api/v1/public/") && rt.Permission != "" {
			t.Errorf("%s %s es publica y exige el permiso %q", rt.Method, rt.Pattern, rt.Permission)
		}
	}
}

// La superficie de salud sigue siendo la que describe el contrato.
func TestLasRutasDePlataformaSonLasDelContrato(t *testing.T) {
	espia := &muxEspia{}
	HandlerFromMuxWithBaseURL(&App{}, espia, "/api/v1")

	montadas := map[string]bool{}
	for _, rt := range rutasDeTodo(t) {
		montadas[rt.Method+" "+rt.Pattern] = true
	}

	for _, patron := range espia.patrones {
		if !montadas[patron] {
			t.Errorf("el contrato declara %q y no esta montada", patron)
		}
	}
}

type muxEspia struct{ patrones []string }

func (m *muxEspia) HandleFunc(patron string, _ func(http.ResponseWriter, *http.Request)) {
	m.patrones = append(m.patrones, patron)
}

func (m *muxEspia) ServeHTTP(http.ResponseWriter, *http.Request) {}
