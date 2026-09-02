package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

type peticion struct {
	Titulo string `json:"titulo"`
	Orden  int    `json:"orden"`
}

func decodificar(t *testing.T, cuerpo string, ct string) (*peticion, *httpx.Problem) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cosas", strings.NewReader(cuerpo))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	var dst peticion
	return &dst, httpx.DecodeJSON(httptest.NewRecorder(), req, &dst)
}

func TestDecodeAceptaUnCuerpoValido(t *testing.T) {
	dst, p := decodificar(t, `{"titulo":"hola","orden":3}`, "application/json")
	if p != nil {
		t.Fatalf("no se esperaba problema: %+v", p)
	}
	if dst.Titulo != "hola" || dst.Orden != 3 {
		t.Errorf("decodificado = %+v", dst)
	}
}

// El caso que mas silencio produce si no se controla: el usuario cree que
// guardo algo que no guardo.
func TestDecodeRechazaCampoDesconocidoYLoNombra(t *testing.T) {
	_, p := decodificar(t, `{"titel":"hola"}`, "application/json")
	if p == nil {
		t.Fatal("un campo desconocido tiene que ser 400, no un silencio")
	}
	if p.Status != http.StatusBadRequest || p.Code != httpx.CodeValidationFailed {
		t.Fatalf("status/code = %d/%s", p.Status, p.Code)
	}
	if len(p.Errors) != 1 || p.Errors[0].Field != "titel" {
		t.Errorf("errors = %+v; tiene que decir CUAL campo sobra", p.Errors)
	}
}

func TestDecodeNombraElCampoDeTipoEquivocado(t *testing.T) {
	_, p := decodificar(t, `{"orden":"tres"}`, "application/json")
	if p == nil {
		t.Fatal("un tipo equivocado tiene que ser 400")
	}
	if len(p.Errors) != 1 || p.Errors[0].Field != "orden" || p.Errors[0].Code != "type" {
		t.Errorf("errors = %+v; tiene que decir que campo y por que", p.Errors)
	}
}

func TestDecodeRechazaDosObjetosPegados(t *testing.T) {
	_, p := decodificar(t, `{"titulo":"a"}{"titulo":"b"}`, "application/json")
	if p == nil {
		t.Fatal("dos objetos en un cuerpo es un cliente mal hecho, no una cortesia")
	}
}

func TestDecodeRechazaCuerpoVacio(t *testing.T) {
	if _, p := decodificar(t, ``, "application/json"); p == nil {
		t.Fatal("un cuerpo vacio tiene que ser 400")
	}
}

func TestDecodeRechazaJsonRoto(t *testing.T) {
	if _, p := decodificar(t, `{"titulo":`, "application/json"); p == nil {
		t.Fatal("un JSON roto tiene que ser 400")
	}
}

func TestDecodeRechazaOtroContentType(t *testing.T) {
	_, p := decodificar(t, `titulo=hola`, "application/x-www-form-urlencoded")
	if p == nil || p.Status != http.StatusUnsupportedMediaType {
		t.Fatalf("se esperaba 415, se obtuvo %+v", p)
	}
}

// Con parametros (charset) el media type sigue siendo application/json.
func TestDecodeAceptaContentTypeConCharset(t *testing.T) {
	if _, p := decodificar(t, `{"titulo":"a"}`, "application/json; charset=utf-8"); p != nil {
		t.Fatalf("el charset no invalida el tipo: %+v", p)
	}
}

// El limite se aplica mientras se lee. Un cuerpo enorme no debe llegar entero a
// memoria solo para descubrir despues que era enorme.
func TestDecodeRechazaCuerpoDemasiadoGrande(t *testing.T) {
	grande := `{"titulo":"` + strings.Repeat("a", httpx.MaxBodyBytes+10) + `"}`
	_, p := decodificar(t, grande, "application/json")
	if p == nil || p.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("se esperaba 413, se obtuvo %+v", p)
	}
}
