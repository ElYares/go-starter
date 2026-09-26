package settings

import (
	"embed"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/esquema"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Los tipos de este modulo —Setting, SettingNuevo, SettingModificacion,
// SettingsPage— YA NO SE ESCRIBEN AQUI. Salen de api/openapi.yaml y viven en
// openapi_gen.go. Ver docs/05-contratos-api.md y la Decision 010.
//
// Lo que queda en este archivo es lo que el contrato no puede expresar: reglas
// de validacion con mensajes propios. OpenAPI declara el `pattern` de la clave,
// pero un `pattern` incumplido no dice al usuario que se acepta y que no.

// Una clave es un identificador, no texto libre: se usa en el frontend como
// nombre de propiedad y aparece en URLs. El patron es el mismo que declara el
// contrato; si uno cambia, tiene que cambiar el otro.
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

// Cada clave tiene su JSON Schema, y el nombre del archivo es la clave. Una
// clave sin esquema no se acepta (Decision 024): la landing lee claves por
// nombre y tiene que poder confiar en su forma. Agregar una clave es agregar un
// archivo aqui.
//
// Los esquemas llevan `title` en cada propiedad porque de ellos sale el
// formulario del dashboard, como con los bloques (Decision 022).
//
//go:embed esquemas/*.json
var esquemasFS embed.FS

// formatoMedio marca un campo que guarda el id de una imagen subida. El
// esquema exige que sea un uuid; que exista lo comprueba el service contra
// media, porque un esquema no puede preguntarle nada a la base.
const formatoMedio = "media-id"

var medioID = esquema.Formato{Nombre: formatoMedio, Validar: func(s string) error {
	_, err := uuid.Parse(s)
	return err
}}

func cargarEsquemas() (map[string]*esquema.Esquema, error) {
	es, err := esquema.Cargar(esquemasFS, "esquemas/*.json", medioID, esquema.FormatoColor)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	return es, nil
}

func claveSinEsquema(k string, es map[string]*esquema.Esquema) *httpx.Problem {
	claves := make([]string, 0, len(es))
	for c := range es {
		claves = append(claves, c)
	}
	slices.Sort(claves)
	return httpx.BadRequest("La clave no esta declarada", httpx.FieldIssue{
		Field: "key", Code: "unknown",
		Message: fmt.Sprintf("No hay esquema para %q. Las que hay: %s", k, strings.Join(claves, ", ")),
	})
}

// validarValor trata el JSON `null` como ausente. El contrato declara `value`
// obligatorio, y una clave de configuracion cuyo valor es null no configura
// nada: distinguir "no lo mandaste" de "lo mandaste vacio" aqui no le sirve a
// nadie.
func validarValor(v interface{}) *httpx.Problem {
	if v == nil {
		return httpx.BadRequest("El valor es obligatorio", httpx.FieldIssue{
			Field: "value", Code: "required", Message: "Este campo es obligatorio",
		})
	}
	return nil
}

// publico resuelve el *bool del contrato. Ausente significa privado: que una
// clave sea publica es una decision explicita, nunca un valor por omision.
func publico(p *bool) bool { return p != nil && *p }
