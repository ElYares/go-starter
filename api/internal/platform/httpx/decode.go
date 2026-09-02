package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxBodyBytes es el tope por omision de un cuerpo JSON. Se aplica MIENTRAS se
// lee, no despues: comprobarlo al final significa haber aceptado ya todo el
// cuerpo, que es justo lo que se queria evitar.
const MaxBodyBytes = 1 << 20 // 1 MiB

// DecodeJSON lee el cuerpo a `dst` y devuelve un Problem listo para escribir.
//
// Tres decisiones que no son de estilo:
//
//   - `DisallowUnknownFields`: un campo desconocido es un 400, no un silencio.
//     Aceptar un `titel` sin decir nada significa que el usuario cree que guardo
//     algo que no guardo
//   - un solo objeto JSON por cuerpo: dos objetos pegados son un error, no una
//     cortesia
//   - los mensajes se redactan aqui, en espanol y por campo, en vez de reenviar
//     el texto de encoding/json, que habla de offsets y tipos de Go
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) *Problem {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mediaType := strings.TrimSpace(strings.Split(ct, ";")[0]); mediaType != "application/json" {
			return New(http.StatusUnsupportedMediaType, CodeUnsupportedMedia,
				"Tipo de contenido no soportado",
				fmt.Sprintf("Se esperaba application/json y llego %q", mediaType))
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return decodeProblem(err)
	}

	// Un segundo valor en el mismo cuerpo casi siempre es un cliente mal hecho
	// mandando dos objetos concatenados. Callarlo esconde el bug del cliente.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BadRequest("El cuerpo debe contener un unico objeto JSON")
	}

	return nil
}

func decodeProblem(err error) *Problem {
	var syntax *json.SyntaxError
	var unmarshalType *json.UnmarshalTypeError
	var maxBytes *http.MaxBytesError

	switch {
	case errors.As(err, &syntax):
		return BadRequest(fmt.Sprintf("El cuerpo no es JSON valido (posicion %d)", syntax.Offset))

	case errors.As(err, &unmarshalType):
		// Este es el caso util de verdad: dice QUE campo y que se esperaba.
		field := unmarshalType.Field
		if field == "" {
			field = "(raiz)"
		}
		return BadRequest("Un campo tiene un tipo que no corresponde", FieldIssue{
			Field:   field,
			Code:    "type",
			Message: fmt.Sprintf("Se esperaba %s", unmarshalType.Type.String()),
		})

	case errors.As(err, &maxBytes):
		return New(http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
			"Cuerpo demasiado grande",
			fmt.Sprintf("El limite es de %d bytes", MaxBodyBytes))

	case errors.Is(err, io.EOF):
		return BadRequest("El cuerpo esta vacio")

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		name := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return BadRequest("El cuerpo trae un campo que no existe", FieldIssue{
			Field:   name,
			Code:    "unknown",
			Message: "Este campo no forma parte de la peticion",
		})

	default:
		return BadRequest("No se pudo leer el cuerpo de la peticion")
	}
}
