package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

func ok(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestElGrupoAplicaSuPrefijo(t *testing.T) {
	r := httpx.NewRouter()
	r.Group("/api/v1", func(r *httpx.Router) {
		r.Group("/public", func(r *httpx.Router) {
			r.Get("/settings", ok)
		})
	})

	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d; los prefijos de grupo no se acumularon", rec.Code)
	}
}

// El middleware del padre se copia, no se comparte: un Use dentro de un grupo
// no puede afectar a lo que se registre fuera de el. Sin esto, agregar un guard
// a una seccion del dashboard protegeria de rebote rutas publicas.
func TestElMiddlewareDeUnGrupoNoSeEscapa(t *testing.T) {
	marca := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Marca", "si")
			next.ServeHTTP(w, req)
		})
	}

	r := httpx.NewRouter()
	r.Group("/dentro", func(r *httpx.Router) {
		r.Use(marca)
		r.Get("/x", ok)
	})
	r.Get("/fuera", ok)

	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dentro/x", nil))
	if rec.Header().Get("X-Marca") != "si" {
		t.Error("el middleware del grupo no se aplico dentro del grupo")
	}

	rec = httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fuera", nil))
	if rec.Header().Get("X-Marca") != "" {
		t.Error("el middleware del grupo se escapo a una ruta de fuera")
	}
}

// El orden es el mismo que en la cadena global: lo mas general envuelve a lo
// mas especifico.
func TestElMiddlewareDelGrupoCorreAntesQueElDeLaRuta(t *testing.T) {
	var orden []string
	marcar := func(nombre string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				orden = append(orden, nombre)
				next.ServeHTTP(w, req)
			})
		}
	}

	r := httpx.NewRouter()
	r.Group("/g", func(r *httpx.Router) {
		r.Use(marcar("grupo"))
		r.Get("/x", ok, httpx.With(marcar("ruta")))
	})

	r.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/g/x", nil))

	if strings.Join(orden, ",") != "grupo,ruta" {
		t.Errorf("orden = %v, se esperaba [grupo ruta]", orden)
	}
}

// Lo que hace posible revisar los permisos al arrancar.
func TestElRouterRecuerdaLoQueSeMonto(t *testing.T) {
	r := httpx.NewRouter()
	r.Group("/api/v1", func(r *httpx.Router) {
		r.Get("/publico", ok)
		r.Post("/privado", ok, httpx.WithPermission("cosa.write"))
	})

	rutas := r.Routes()
	if len(rutas) != 2 {
		t.Fatalf("rutas = %d, se esperaban 2", len(rutas))
	}

	porPatron := map[string]httpx.Route{}
	for _, rt := range rutas {
		porPatron[rt.Pattern] = rt
	}

	if p := porPatron["/api/v1/publico"].Permission; p != "" {
		t.Errorf("la ruta publica quedo con permiso %q", p)
	}
	if p := porPatron["/api/v1/privado"].Permission; p != "cosa.write" {
		t.Errorf("permiso = %q, se esperaba cosa.write", p)
	}
	if m := porPatron["/api/v1/privado"].Method; m != http.MethodPost {
		t.Errorf("metodo = %q", m)
	}
}

// Un patron sin barra inicial hace que ServeMux entre en panico con un mensaje
// que no dice de que modulo salio. Mejor fallar nombrando la ruta.
func TestUnPatronSinBarraFallaConMensajeUtil(t *testing.T) {
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("se esperaba un panic")
		}
		if !strings.Contains(rec.(string), "sin-barra") {
			t.Errorf("el mensaje tiene que nombrar la ruta culpable: %v", rec)
		}
	}()

	r := httpx.NewRouter()
	r.Get("sin-barra", ok)
}

// El comodin de NewRouter es lo que mantiene la promesa de forma unica.
func TestLoNoReclamadoSaleComoProblemJson(t *testing.T) {
	r := httpx.NewRouter()
	r.Get("/existe", ok)

	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q", ct)
	}
}
