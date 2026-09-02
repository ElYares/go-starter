// Package validate traduce las reglas de validacion a la forma de error de la
// API: una lista de FieldIssue con `field` en el nombre JSON, `code` estable y
// `message` en espanol.
//
// El porque de traducir en vez de reenviar: la libreria contesta en ingles y
// nombrando el campo de Go (`DisplayName`), no el del contrato
// (`displayName`). Un cliente no puede resaltar el campo equivocado en un
// formulario.
package validate

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

var (
	once sync.Once
	v    *validator.Validate
)

func instance() *validator.Validate {
	once.Do(func() {
		v = validator.New(validator.WithRequiredStructEnabled())
		// El nombre que ve el cliente es el del JSON, no el del struct de Go.
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
			if name == "-" || name == "" {
				return f.Name
			}
			return name
		})
	})
	return v
}

// Struct valida y devuelve un Problem listo para escribir, o nil.
func Struct(dst any) *httpx.Problem {
	err := instance().Struct(dst)
	if err == nil {
		return nil
	}

	var invalid *validator.InvalidValidationError
	if ok := asInvalid(err, &invalid); ok {
		// Programador, no usuario: se le paso algo que no es struct.
		return httpx.Internal()
	}

	var errs validator.ValidationErrors
	if !asValidation(err, &errs) {
		return httpx.BadRequest("No se pudo validar la peticion")
	}

	issues := make([]httpx.FieldIssue, 0, len(errs))
	for _, e := range errs {
		issues = append(issues, httpx.FieldIssue{
			Field:   fieldPath(e),
			Code:    e.Tag(),
			Message: message(e),
		})
	}

	return httpx.BadRequest(
		fmt.Sprintf("El cuerpo tiene %d campo(s) invalido(s)", len(issues)), issues...)
}

// fieldPath deja `direccion.calle` en vez de `Peticion.Direccion.Calle`: el
// cliente necesita la ruta dentro del JSON que mando, no la del struct.
func fieldPath(e validator.FieldError) string {
	ns := e.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		return ns[i+1:]
	}
	return e.Field()
}

func message(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "Este campo es obligatorio"
	case "email":
		return "No es un correo valido"
	case "min":
		return fmt.Sprintf("El minimo es %s", e.Param())
	case "max":
		return fmt.Sprintf("El maximo es %s", e.Param())
	case "len":
		return fmt.Sprintf("Debe tener exactamente %s", e.Param())
	case "oneof":
		return fmt.Sprintf("Solo se aceptan: %s", strings.ReplaceAll(e.Param(), " ", ", "))
	case "url":
		return "No es una URL valida"
	case "uuid", "uuid4", "uuid7":
		return "No es un identificador valido"
	case "gte":
		return fmt.Sprintf("Debe ser mayor o igual a %s", e.Param())
	case "lte":
		return fmt.Sprintf("Debe ser menor o igual a %s", e.Param())
	case "slug":
		return "Solo minusculas, numeros y guiones"
	default:
		// Un tag sin mensaje propio no puede quedarse sin texto: el `code` viaja
		// igual, asi que el cliente puede traducirlo por su cuenta.
		return fmt.Sprintf("No cumple la regla %q", e.Tag())
	}
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	e, ok := err.(*validator.InvalidValidationError)
	if ok {
		*target = e
	}
	return ok
}

func asValidation(err error, target *validator.ValidationErrors) bool {
	e, ok := err.(validator.ValidationErrors)
	if ok {
		*target = e
	}
	return ok
}
