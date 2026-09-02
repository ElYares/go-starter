package httpx_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

func TestUnPanicoNoTumbaElProcesoYSaleComoProblem(t *testing.T) {
	var log strings.Builder
	logger := slog.New(slog.NewJSONHandler(&log, nil))

	h := observ.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("la base dijo que no: usuario=juan token=secreto")
		}),
		observ.TraceID,
		httpx.Recover(logger),
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("estado = %d, se esperaba 500", rec.Code)
	}

	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("el 500 tiene que ser problem+json: %v", err)
	}
	if p.Code != httpx.CodeInternal || p.TraceID == "" {
		t.Errorf("code=%q traceId=%q", p.Code, p.TraceID)
	}

	// Lo que el cliente NO puede ver.
	if strings.Contains(rec.Body.String(), "secreto") {
		t.Error("el cuerpo del 500 filtro el mensaje del panico")
	}
	// Y lo que el log SI tiene que tener, con el mismo traceId.
	if !strings.Contains(log.String(), "secreto") {
		t.Error("el panico no quedo registrado: el detalle se pierde en los dos lados")
	}
	if !strings.Contains(log.String(), p.TraceID) {
		t.Error("el log no lleva el traceId de la respuesta: no se pueden correlacionar")
	}
	if !strings.Contains(log.String(), "stack") {
		t.Error("el log no trae stack")
	}
}

// http.ErrAbortHandler es la forma que tiene el propio net/http de decir "el
// cliente se fue". Tragarselo convertiria una desconexion en un 500 inventado.
func TestElAbortHandlerSeDejaPasar(t *testing.T) {
	h := httpx.Recover(slog.New(slog.NewJSONHandler(io.Discard, nil)))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}))

	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Errorf("recover() = %v; ErrAbortHandler tiene que seguir subiendo", rec)
		}
	}()

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Error("no se propago el panico")
}
