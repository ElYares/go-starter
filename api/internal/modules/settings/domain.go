package settings

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Setting es una clave de configuracion. `Value` viaja como JSON crudo: el
// esquema por clave se valida en la fase 4, y hasta entonces no tiene sentido
// inventarle un tipo de Go que no representa nada.
type Setting struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	IsPublic  bool            `json:"isPublic"`
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
	// UpdatedBy es nulo en las filas que sembro la migracion inicial: nadie las
	// escribio. De ahi en adelante lo llena platform/audit y no vuelve a serlo.
	UpdatedBy *string `json:"updatedBy"`
}

// entrada es el DTO de alta. Es explicito y separado de Setting a proposito:
// decodificar el cuerpo directamente sobre la entidad es mass assignment, y
// regalaria `version`, `updatedAt` y `updatedBy` al cliente.
type entrada struct {
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value"`
	IsPublic bool            `json:"isPublic"`
}

// modificacion es el DTO de reemplazo. No trae `key`: la clave es la identidad
// del recurso y viaja en la ruta. Aceptarla en el cuerpo abriria la puerta a un
// PUT que renombra la fila que estaba editando otro.
type modificacion struct {
	Value    json.RawMessage `json:"value"`
	IsPublic bool            `json:"isPublic"`
}

// Una clave es un identificador, no texto libre: se usa en el frontend como
// nombre de propiedad y aparece en URLs.
var claveValida = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

func validarClave(k string) *httpx.Problem {
	switch {
	case k == "":
		return httpx.BadRequest("La clave es obligatoria", httpx.FieldIssue{
			Field: "key", Code: "required", Message: "Este campo es obligatorio",
		})
	case len(k) > 120:
		return httpx.BadRequest("La clave es demasiado larga", httpx.FieldIssue{
			Field: "key", Code: "max", Message: "El maximo es 120 caracteres",
		})
	case !claveValida.MatchString(k):
		return httpx.BadRequest("La clave tiene caracteres que no se aceptan", httpx.FieldIssue{
			Field: "key", Code: "format",
			Message: "Solo minusculas, numeros y los separadores . _ - entre ellos",
		})
	}
	return nil
}

func validarValor(v json.RawMessage) *httpx.Problem {
	if len(v) == 0 {
		return httpx.BadRequest("El valor es obligatorio", httpx.FieldIssue{
			Field: "value", Code: "required", Message: "Este campo es obligatorio",
		})
	}
	return nil
}
