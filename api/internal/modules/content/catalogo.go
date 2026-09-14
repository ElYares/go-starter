package content

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// El catalogo de bloques: un archivo JSON Schema por tipo, y el nombre del
// archivo es el `type`. Es lo que un fork edita para cambiar el lenguaje visual
// del sitio: agregar un tipo es agregar un archivo aqui y su componente en
// web/app/shared/blocks/. Ver la Decision 006.
//
// Los esquemas se escriben en JSON Schema y no como structs de Go porque el
// otro lado —el editor del dashboard— tambien los va a necesitar, y un struct
// de Go no lo puede leer nadie mas.
//
//go:embed bloques/*.json
var bloquesFS embed.FS

type catalogo struct {
	esquemas map[string]*jsonschema.Schema
}

// cargarCatalogo compila todos los esquemas al arrancar. Un esquema mal escrito
// es un error de programacion, no de la peticion: tiene que impedir levantar,
// no aparecer como un 500 la primera vez que alguien guarda ese bloque.
func cargarCatalogo(fsys fs.FS) (*catalogo, error) {
	archivos, err := fs.Glob(fsys, "bloques/*.json")
	if err != nil {
		return nil, err
	}
	if len(archivos) == 0 {
		return nil, fmt.Errorf("content: el catalogo de bloques esta vacio")
	}

	c := jsonschema.NewCompiler()
	cat := &catalogo{esquemas: make(map[string]*jsonschema.Schema, len(archivos))}

	for _, archivo := range archivos {
		crudo, err := fs.ReadFile(fsys, archivo)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(crudo))
		if err != nil {
			return nil, fmt.Errorf("content: %s no es JSON: %w", archivo, err)
		}

		// Una URL propia por tipo. Sin esquema absoluto, el compilador intenta
		// resolverla como ruta del sistema de archivos del proceso.
		url := "mem://content/" + archivo
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("content: %s: %w", archivo, err)
		}
		esquema, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("content: el esquema %s no compila: %w", archivo, err)
		}

		cat.esquemas[strings.TrimSuffix(path.Base(archivo), ".json")] = esquema
	}

	return cat, nil
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
		esquema, existe := c.esquemas[b.Type]
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

		issues = append(issues, validarProps(esquema, b.Props, base+".props")...)
	}

	return issues
}

// validarProps pasa las props por JSON antes de validarlas. El validador espera
// los valores tal como los deja su propio decodificador (numeros como
// json.Number); darle directamente lo que dejo encoding/json funciona hoy y
// deja de funcionar el dia que alguien construya las props a mano con un int.
func validarProps(esquema *jsonschema.Schema, props map[string]interface{}, base string) []httpx.FieldIssue {
	crudo, err := json.Marshal(props)
	if err != nil {
		return []httpx.FieldIssue{{Field: base, Code: "type", Message: "No se pudo leer como JSON"}}
	}
	valor, err := jsonschema.UnmarshalJSON(bytes.NewReader(crudo))
	if err != nil {
		return []httpx.FieldIssue{{Field: base, Code: "type", Message: "No se pudo leer como JSON"}}
	}

	err = esquema.Validate(valor)
	if err == nil {
		return nil
	}
	verr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []httpx.FieldIssue{{Field: base, Code: "invalid", Message: "No cumple el esquema del bloque"}}
	}

	var issues []httpx.FieldIssue
	for _, hoja := range hojas(verr) {
		issues = append(issues, traducir(hoja, base)...)
	}
	return issues
}

// hojas se queda con los errores concretos. Los de arriba del arbol son
// agrupadores —"el esquema no se cumplio"— y no dicen que campo arreglar.
func hojas(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, hojas(c)...)
	}
	return out
}

// traducir convierte un error del validador a la forma de la API: el campo con
// la ruta dentro del JSON que mando el cliente, un `code` estable y el mensaje
// en espanol. Los codigos son los mismos que usa platform/validate, para que el
// frontend no tenga dos vocabularios.
func traducir(e *jsonschema.ValidationError, base string) []httpx.FieldIssue {
	campo := ruta(base, e.InstanceLocation)

	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		issues := make([]httpx.FieldIssue, 0, len(k.Missing))
		for _, falta := range k.Missing {
			issues = append(issues, obligatorio(campo+"."+falta))
		}
		return issues

	case *kind.AdditionalProperties:
		issues := make([]httpx.FieldIssue, 0, len(k.Properties))
		for _, sobra := range k.Properties {
			issues = append(issues, httpx.FieldIssue{
				Field: campo + "." + sobra, Code: "unknown",
				Message: "Este campo no forma parte del bloque",
			})
		}
		return issues

	case *kind.Type:
		return uno(campo, "type", "Se esperaba un valor de tipo "+strings.Join(k.Want, " o "))
	case *kind.MinLength:
		if k.Want == 1 {
			return uno(campo, "required", "No puede estar vacio")
		}
		return uno(campo, "min", fmt.Sprintf("El minimo es %d caracteres", k.Want))
	case *kind.MaxLength:
		return uno(campo, "max", fmt.Sprintf("El maximo es %d caracteres", k.Want))
	case *kind.MinItems:
		return uno(campo, "min", fmt.Sprintf("Hacen falta al menos %d elementos", k.Want))
	case *kind.MaxItems:
		return uno(campo, "max", fmt.Sprintf("El maximo es %d elementos", k.Want))
	case *kind.Pattern:
		return uno(campo, "format", "No tiene el formato que acepta este campo")
	case *kind.Enum:
		return uno(campo, "oneof", "No es uno de los valores que acepta este campo")
	default:
		// Un tipo de error sin mensaje propio no se queda sin campo: el editor
		// sigue sabiendo que resaltar, aunque el texto sea generico.
		return uno(campo, "invalid", "No cumple el esquema del bloque")
	}
}

// ruta arma `blocks[0].props.items[2].title` a partir de la ubicacion que da
// el validador, que viene partida en trozos y sin distinguir indices de claves.
// Un trozo numerico se escribe como indice: las props del catalogo no usan
// claves numericas, y si un fork las usa, el campo sigue siendo encontrable.
func ruta(base string, ubicacion []string) string {
	var b strings.Builder
	b.WriteString(base)
	for _, trozo := range ubicacion {
		if _, err := strconv.Atoi(trozo); err == nil {
			b.WriteString("[" + trozo + "]")
			continue
		}
		b.WriteString("." + trozo)
	}
	return b.String()
}

func obligatorio(campo string) httpx.FieldIssue {
	return httpx.FieldIssue{Field: campo, Code: "required", Message: "Este campo es obligatorio"}
}

func uno(campo, code, mensaje string) []httpx.FieldIssue {
	return []httpx.FieldIssue{{Field: campo, Code: code, Message: mensaje}}
}
