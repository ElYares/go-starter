package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// La promesa del molde es que TODO error de la API tiene la misma forma. El
// 404 de omision de net/http es texto plano y la rompe sin que nadie lo note,
// porque un 404 "se ve bien" en el navegador.
func TestUnaRutaInexistenteContestaProblemJson(t *testing.T) {
	a := &App{log: loggerDePrueba()}
	srv := a.Handler()

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/no-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q; el 404 de omision de Go es text/plain", ct)
	}

	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("cuerpo no JSON: %q", rec.Body.String())
	}
	if p.Code != httpx.CodeNotFound || p.TraceID == "" {
		t.Errorf("code=%q traceId=%q", p.Code, p.TraceID)
	}
}

func TestHealthzRespondeSinTocarLaBase(t *testing.T) {
	// pool en nil a proposito: si healthz consultara la base, esto reventaria.
	a := &App{log: loggerDePrueba()}

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d, se esperaba 200", rec.Code)
	}
}
