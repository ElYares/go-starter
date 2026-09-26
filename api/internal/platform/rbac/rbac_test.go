package rbac_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

func routerCon(actor *rbac.Actor) http.Handler {
	r := httpx.NewRouter()
	r.Get("/protegido", func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteJSON(w, req, http.StatusOK, map[string]string{"ok": "si"})
	}, rbac.Require("cosa.write"))

	// El actor lo inyecta una prueba, nunca un bypass de desarrollo: en la
	// fase 2 lo llenara el middleware de sesion desde la cookie.
	inyectar := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if actor != nil {
				req = req.WithContext(rbac.WithActor(req.Context(), *actor))
			}
			next.ServeHTTP(w, req)
		})
	}

	return observ.Chain(r.Handler(), observ.TraceID, inyectar)
}

func pedir(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protegido", nil))
	return rec
}

// Sin sesion es 401, no 403: son preguntas distintas y confundirlas hace que el
// frontend lleve al login a quien si tiene sesion, o al reves.
func TestSinActorResponde401(t *testing.T) {
	if got := pedir(routerCon(nil)).Code; got != http.StatusUnauthorized {
		t.Errorf("estado = %d, se esperaba 401", got)
	}
}

func TestConActorSinElPermisoResponde403(t *testing.T) {
	actor := rbac.Actor{ID: "u1", Permissions: []string{"otra.cosa"}}
	if got := pedir(routerCon(&actor)).Code; got != http.StatusForbidden {
		t.Errorf("estado = %d, se esperaba 403", got)
	}
}

func TestConElPermisoPasa(t *testing.T) {
	actor := rbac.Actor{ID: "u1", Permissions: []string{"cosa.write"}}
	if got := pedir(routerCon(&actor)).Code; got != http.StatusOK {
		t.Errorf("estado = %d, se esperaba 200", got)
	}
}

// El segundo valor de ActorFrom importa: sin el, un actor vacio y "no hay
// sesion" serian indistinguibles, y eso convierte un 401 en un 403.
func TestActorFromDistingueAusenteDeVacio(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, ok := rbac.ActorFrom(req.Context()); ok {
		t.Error("sin actor inyectado, ok tiene que ser false")
	}

	ctx := rbac.WithActor(req.Context(), rbac.Actor{})
	if _, ok := rbac.ActorFrom(ctx); !ok {
		t.Error("con un actor vacio inyectado, ok tiene que ser true")
	}
}

func TestVerifyRoutesIgnoraLasRutasPublicas(t *testing.T) {
	reg, err := rbac.NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	rutas := []httpx.Route{{Method: "GET", Pattern: "/api/v1/public/x", Permission: ""}}
	if err := rbac.VerifyRoutes(rutas, reg); err != nil {
		t.Errorf("una ruta sin permiso es publica y no tiene que fallar: %v", err)
	}
}

// Sin area, la vista de roles pondria el permiso bajo un titulo vacio. Es un
// olvido del modulo que lo declara, y se ve mejor al arrancar que en pantalla.
func TestNewRegistryRechazaUnPermisoSinArea(t *testing.T) {
	_, err := rbac.NewRegistry([]rbac.Permission{
		{Key: "cosa.read", Desc: "Ver", Area: "Cosas"},
		{Key: "cosa.write", Desc: "Escribir"},
	})
	if err == nil {
		t.Fatal("un permiso sin area tenia que impedir el registro")
	}
	if !strings.Contains(err.Error(), "cosa.write") {
		t.Errorf("el error tiene que nombrar el permiso culpable; dijo: %v", err)
	}
}

// routerConSesion monta una ruta protegida SOLO por RequireSession, que es como
// va `GET /auth/me`.
func routerConSesion(actor *rbac.Actor) http.Handler {
	r := httpx.NewRouter()
	r.Get("/mio", func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteJSON(w, req, http.StatusOK, map[string]string{"ok": "si"})
	}, rbac.RequireSession())

	inyectar := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if actor != nil {
				req = req.WithContext(rbac.WithActor(req.Context(), *actor))
			}
			next.ServeHTTP(w, req)
		})
	}

	return observ.Chain(r.Handler(), observ.TraceID, inyectar)
}

func pedirMio(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mio", nil))
	return rec
}

// RequireSession exige sesion y nada mas: es lo que protege `GET /auth/me`, que
// no tiene un permiso con nombre que pedir porque todo el mundo puede leer su
// propio perfil. Un actor SIN un solo permiso tiene que pasar.
func TestRequireSessionDejaPasarAUnActorSinPermisos(t *testing.T) {
	rec := pedirMio(routerConSesion(&rbac.Actor{ID: "u-1"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d; un actor sin permisos tiene que poder leer su perfil", rec.Code)
	}
}

// Y al anonimo lo corta con 401, no con 403: no es que le falte un permiso, es
// que no hay sesion.
func TestRequireSessionRechazaAlAnonimoCon401(t *testing.T) {
	rec := pedirMio(routerConSesion(nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d, se esperaba 401", rec.Code)
	}
}

// RequireSession NO anota permiso en la ruta. Es la razon por la que
// `/auth/me` tiene que estar escrita como excepcion en la prueba de contrato de
// app: desde alli se ve igual que una ruta sin proteger.
func TestRequireSessionNoAnotaNingunPermisoEnLaRuta(t *testing.T) {
	r := httpx.NewRouter()
	r.Get("/mio", func(http.ResponseWriter, *http.Request) {}, rbac.RequireSession())

	rutas := r.Routes()
	if len(rutas) != 1 {
		t.Fatalf("rutas = %d", len(rutas))
	}
	if rutas[0].Permission != "" {
		t.Errorf("Permission = %q; RequireSession no pide ningun permiso con nombre", rutas[0].Permission)
	}
}
