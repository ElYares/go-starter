package content

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/elyares/go-starter/api/internal/platform/esquema"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// El catalogo de bloques: un archivo JSON Schema por tipo, y el nombre del
// archivo es el `type`. Es lo que un fork edita para cambiar el lenguaje visual
// del sitio: agregar un tipo es agregar un archivo aqui y su componente en
// web/app/shared/blocks/. Ver la Decision 006.
//
// Compilarlos y traducir sus errores es de platform/esquema; aqui queda lo que
// es de los bloques: el id, el tipo y las props.
//
//go:embed bloques/*.json
var bloquesFS embed.FS

type catalogo struct {
	esquemas map[string]*esquema.Esquema
}

// cargarCatalogo compila todos los esquemas al arrancar: uno roto impide
// levantar. Un catalogo vacio tambien, porque ninguna pagina podria guardarse.
func cargarCatalogo(fsys fs.FS) (*catalogo, error) {
	esquemas, err := esquema.Cargar(fsys, "bloques/*.json")
	if err != nil {
		return nil, fmt.Errorf("content: %w", err)
	}
	if len(esquemas) == 0 {
		return nil, fmt.Errorf("content: el catalogo de bloques esta vacio")
	}
	return &catalogo{esquemas: esquemas}, nil
}

func (c *catalogo) tipos() []string {
	tipos := make([]string, 0, len(c.esquemas))
	for t := range c.esquemas {
		tipos = append(tipos, t)
	}
	slices.Sort(tipos)
	return tipos
}

// validar revisa TODOS los bloques y devuelve todos los problemas juntos. El
// editor necesita marcar cada campo mal puesto de una vez; devolver solo el
// primero obliga a guardar diez veces para descubrir diez errores.
//
// Cada problema nombra el bloque por su indice —`blocks[2].props.title`— porque
// es lo que el editor puede resaltar sin buscar.
func (c *catalogo) validar(bloques []Bloque) []httpx.FieldIssue {
	var issues []httpx.FieldIssue
	vistos := make(map[string]int, len(bloques))

	for i, b := range bloques {
		base := fmt.Sprintf("blocks[%d]", i)

		switch {
		case b.Id == "":
			issues = append(issues, obligatorio(base+".id"))
		case len([]rune(b.Id)) > 64:
			issues = append(issues, httpx.FieldIssue{
				Field: base + ".id", Code: "max", Message: "El maximo es 64 caracteres",
			})
		default:
			// Dos bloques con el mismo id hacen que el editor mueva uno creyendo
			// mover el otro.
			if previo, repetido := vistos[b.Id]; repetido {
				issues = append(issues, httpx.FieldIssue{
					Field: base + ".id", Code: "duplicate",
					Message: fmt.Sprintf("Ya lo usa el bloque %d", previo),
				})
			} else {
				vistos[b.Id] = i
			}
		}

		if b.Type == "" {
			issues = append(issues, obligatorio(base+".type"))
			continue
		}
		e, existe := c.esquemas[b.Type]
		if !existe {
			issues = append(issues, httpx.FieldIssue{
				Field: base + ".type", Code: "unknown",
				Message: fmt.Sprintf("No existe el tipo de bloque %q. Los que hay: %s",
					b.Type, strings.Join(c.tipos(), ", ")),
			})
			continue
		}

		if b.Props == nil {
			issues = append(issues, obligatorio(base+".props"))
			continue
		}

		issues = append(issues, e.Validar(b.Props, base+".props")...)
	}

	return issues
}

func obligatorio(campo string) httpx.FieldIssue { return esquema.Obligatorio(campo) }
