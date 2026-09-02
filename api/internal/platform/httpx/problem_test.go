package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// conTraceID envuelve el handler igual que la cadena real, para que el traceId
// exista. Sin esto las pruebas afirmarian sobre un mundo que no es el de
// produccion.
func conTraceID(h http.HandlerFunc) http.Handler {
	return observ.TraceID(h)
}

func TestProblemTraeCodigoTraceIdEInstance(t *testing.T) {
	h := conTraceID(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblem(w, r, httpx.NotFound())
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cosas/7", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, se esperaba application/problem+json", ct)
	}

	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v", err)
	}
	if p.Code != httpx.CodeNotFound {
		t.Errorf("code = %q, se esperaba %q", p.Code, httpx.CodeNotFound)
	}
	if p.TraceID == "" {
		t.Error("traceId vacio: sin el, un error reportado por un usuario no se puede encontrar en el log")
	}
	if p.Instance != "/api/v1/cosas/7" {
		t.Errorf("instance = %q, se esperaba la ruta pedida", p.Instance)
	}
	if p.Type != "about:blank" {
		t.Errorf("type = %q, se esperaba about:blank por omision", p.Type)
	}
}

func TestElTraceIdDeLaRespuestaEsElDeLaCabecera(t *testing.T) {
	h := conTraceID(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblem(w, r, httpx.Forbidden())
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(observ.HeaderTraceID, "trace-de-prueba")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var p httpx.Problem
	_ = json.Unmarshal(rec.Body.Bytes(), &p)

	if p.TraceID != "trace-de-prueba" {
		t.Errorf("traceId del cuerpo = %q; el que manda el cliente tiene que propagarse", p.TraceID)
	}
	if got := rec.Header().Get(observ.HeaderTraceID); got != "trace-de-prueba" {
		t.Errorf("traceId de la cabecera = %q; tiene que ser el mismo del cuerpo", got)
	}
}

// El 500 es el unico que no puede filtrar nada: ni el mensaje del error, ni SQL,
// ni rutas del servidor.
func TestElInternalNoFiltraNada(t *testing.T) {
	p := httpx.Internal()
	cuerpo := p.Title + " " + p.Detail

	for _, prohibido := range []string{"sql", "pgx", "panic", "goroutine", "/workspace"} {
		if strings.Contains(strings.ToLower(cuerpo), prohibido) {
			t.Errorf("el 500 menciona %q: %s", prohibido, cuerpo)
		}
	}
	if p.Status != http.StatusInternalServerError || p.Code != httpx.CodeInternal {
		t.Errorf("status/code = %d/%s", p.Status, p.Code)
	}
}

// Un Problem tiene que poder viajar como error desde un service hasta el
// handler sin perder el codigo por el camino.
func TestProblemSirveComoError(t *testing.T) {
	var err error = httpx.Conflict("la version cambio")
	p, ok := err.(*httpx.Problem)
	if !ok {
		t.Fatal("un *Problem devuelto como error tiene que poder recuperarse con un type assertion")
	}
	if p.Code != httpx.CodeConflict {
		t.Errorf("code = %q", p.Code)
	}
}
