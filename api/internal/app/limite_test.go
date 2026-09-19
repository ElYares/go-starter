package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/limite"
)

// El tope por IP esta en la cadena de produccion, no solo en su paquete: sin
// esta prueba, quitarlo de Handler() compilaria y todas las del paquete limite
// seguirian en verde.
func TestLaCadenaAplicaElTopeDeAuth(t *testing.T) {
	a := appDePrueba(t, modulosDePrueba(t)...)
	h := a.Handler()

	pedirMe := func() int {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		r.Header.Set("X-Forwarded-For", "203.0.113.7")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	for i := range limite.TopeAuth {
		if code := pedirMe(); code != http.StatusUnauthorized {
			t.Fatalf("peticion %d: estado = %d, se esperaba el 401 de sin sesion", i+1, code)
		}
	}
	if code := pedirMe(); code != http.StatusTooManyRequests {
		t.Errorf("pasado el tope: estado = %d, se esperaba 429", code)
	}
}

// Y en el orden correcto: el SSR se acredita ANTES de contar. Al reves, todas
// las visitas de la landing contarian contra el contenedor web.
func TestEnLaCadenaElSSRSeAcreditaAntesDeContar(t *testing.T) {
	a := appDePrueba(t, modulosDePrueba(t)...)
	h := a.Handler()

	for i := range limite.TopeAuth + 5 {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		r.RemoteAddr = "172.18.0.5:41234"
		r.Header.Set(httpx.CabeceraSecretoSSR, configDePrueba().SSRSecret)
		r.Header.Set(httpx.CabeceraIPDelSSR, fmt.Sprintf("203.0.113.%d", i+1))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("visitante %d via SSR recibio 429: cuentan contra el contenedor web", i+1)
		}
	}
}
