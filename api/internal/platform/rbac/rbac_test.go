package rbac_test

import (
	"net/http"
	"net/http/httptest"
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
