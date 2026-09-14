package httpx

import (
	"fmt"
	"strconv"
	"strings"
)

// ETag es la version entre comillas. Es lo que el cliente devuelve en If-Match
// sin tener que entender que por dentro es un contador.
//
// Vive en la plataforma y no en cada modulo porque todo recurso editable del
// molde la usa (docs/04-reglas-de-crud.md seccion 4), y dos copias que difieran
// en las comillas producen un 400 en un modulo y no en el otro.
func ETag(version int) string { return fmt.Sprintf("%q", strconv.Itoa(version)) }

// VersionFromIfMatch lee la version que el cliente creia estar editando.
func VersionFromIfMatch(crudo string) (int, *Problem) {
	crudo = strings.TrimSpace(crudo)

	// `*` significa "cualquiera": es una sobrescritura incondicional, que es
	// exactamente lo que aqui no se quiere permitir.
	if crudo == "*" {
		return 0, BadRequest("If-Match no acepta *", FieldIssue{
			Field: "If-Match", Code: "format",
			Message: "Manda el ETag concreto que devolvio la lectura",
		})
	}

	version, err := strconv.Atoi(strings.Trim(strings.TrimPrefix(crudo, "W/"), `"`))
	if err != nil || version < 1 {
		return 0, BadRequest("If-Match no tiene la forma de un ETag de este recurso",
			FieldIssue{
				Field: "If-Match", Code: "format",
				Message: `Se esperaba el ETag devuelto por la lectura, por ejemplo "7"`,
			})
	}

	return version, nil
}
