package validate_test

import (
	"net/http"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/validate"
)

type direccion struct {
	Calle string `json:"calle" validate:"required"`
}

type alta struct {
	Correo    string    `json:"correo" validate:"required,email"`
	Nombre    string    `json:"nombre" validate:"required,max=5"`
	Rol       string    `json:"rol" validate:"required,oneof=admin staff"`
	Direccion direccion `json:"direccion"`
}

func issuePorCampo(p *httpx.Problem, campo string) *httpx.FieldIssue {
	for i := range p.Errors {
		if p.Errors[i].Field == campo {
			return &p.Errors[i]
		}
	}
	return nil
}

func TestValidoNoProduceProblema(t *testing.T) {
	p := validate.Struct(alta{
		Correo: "a@b.com", Nombre: "juan", Rol: "admin",
		Direccion: direccion{Calle: "x"},
	})
	if p != nil {
		t.Fatalf("no se esperaba problema: %+v", p.Errors)
	}
}

// El punto de todo el paquete: el campo se nombra como en el JSON, no como en
// Go. Un cliente resalta `correo`, no `Correo`.
func TestElCampoSeNombraComoEnElJson(t *testing.T) {
	p := validate.Struct(alta{Nombre: "juan", Rol: "admin", Direccion: direccion{Calle: "x"}})
	if p == nil {
		t.Fatal("un correo vacio tiene que fallar")
	}
	if p.Status != http.StatusBadRequest || p.Code != httpx.CodeValidationFailed {
		t.Fatalf("status/code = %d/%s", p.Status, p.Code)
	}
	if issuePorCampo(p, "correo") == nil {
		t.Errorf("se esperaba el campo 'correo' en minuscula; llego %+v", p.Errors)
	}
}

func TestElCampoAnidadoLlevaSuRutaEnElJson(t *testing.T) {
	p := validate.Struct(alta{Correo: "a@b.com", Nombre: "juan", Rol: "admin"})
	if p == nil {
		t.Fatal("una calle vacia tiene que fallar")
	}
	if issuePorCampo(p, "direccion.calle") == nil {
		t.Errorf("se esperaba 'direccion.calle', sin el nombre del struct raiz; llego %+v", p.Errors)
	}
}

func TestVariosCamposInvalidosSalenTodosDeUnaVez(t *testing.T) {
	p := validate.Struct(alta{Nombre: "demasiado largo", Rol: "invitado", Direccion: direccion{Calle: "x"}})
	if p == nil {
		t.Fatal("se esperaban fallos")
	}
	if len(p.Errors) != 3 {
		t.Fatalf("errors = %d, se esperaban 3 (correo, nombre, rol): %+v", len(p.Errors), p.Errors)
	}
	// Descubrirlos de uno en uno son tres viajes del formulario.
	for campo, code := range map[string]string{"correo": "required", "nombre": "max", "rol": "oneof"} {
		got := issuePorCampo(p, campo)
		if got == nil {
			t.Errorf("falta el campo %q", campo)
			continue
		}
		if got.Code != code {
			t.Errorf("%s: code = %q, se esperaba %q", campo, got.Code, code)
		}
		if got.Message == "" {
			t.Errorf("%s: mensaje vacio", campo)
		}
	}
}

// El code es lo estable; el message es para humanos y esta en espanol, no en el
// ingles de la libreria.
func TestElMensajeEstaEnEspanol(t *testing.T) {
	p := validate.Struct(alta{Rol: "admin", Correo: "a@b.com", Direccion: direccion{Calle: "x"}})
	issue := issuePorCampo(p, "nombre")
	if issue == nil {
		t.Fatal("falta el campo nombre")
	}
	if issue.Message != "Este campo es obligatorio" {
		t.Errorf("message = %q; se esperaba el texto en espanol, no el de la libreria", issue.Message)
	}
}
