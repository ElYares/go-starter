// Package esquema compila JSON Schemas y traduce sus errores a la forma de la
// API.
//
// Lo usan los modulos que guardan JSON con forma declarada: los bloques de
// `content` y las claves de `settings`. Los esquemas se escriben en JSON Schema
// y no como structs de Go porque el otro lado —los formularios del dashboard—
// tambien los lee (Decision 022), y un struct de Go no lo puede leer nadie mas.
package esquema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Esquema es un JSON Schema ya compilado.
type Esquema struct {
	s *jsonschema.Schema
}

// Formato es un `format` propio. Solo se aplica a strings: el resto de los
// tipos lo ignoran, como pide la especificacion.
type Formato struct {
	Nombre  string
	Validar func(string) error
}

// FormatoColor es un color hexadecimal de seis cifras, `#2f6df6`: lo que
// entiende un `<input type="color">`. Es un formato y no un `pattern` para que
// el formulario del dashboard sepa que pintar una muestra y un selector; un
// `pattern` solo dice que letras caben. El modulo que lo quiera lo pasa a
// Cargar, como cualquier otro formato propio.
var FormatoColor = Formato{Nombre: "color", Validar: func(s string) error {
	if !esColorHex(s) {
		return fmt.Errorf("%q no es un color #rrggbb", s)
	}
	return nil
}}

func esColorHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// Cargar compila cada archivo de fsys que casa con patron, y lo guarda por su
// nombre sin `.json`. Un esquema mal escrito es un error de programacion, no de
// la peticion: tiene que impedir arrancar, no aparecer como un 500 la primera
// vez que alguien guarda.
//
// Los formatos que se pasan se EXIGEN: un `format` desconocido o incumplido es
// un error de validacion, no una anotacion que se ignora.
func Cargar(fsys fs.FS, patron string, formatos ...Formato) (map[string]*Esquema, error) {
	archivos, err := fs.Glob(fsys, patron)
	if err != nil {
		return nil, err
	}

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, f := range formatos {
		c.RegisterFormat(&jsonschema.Format{Name: f.Nombre, Validate: soloStrings(f.Validar)})
	}

	out := make(map[string]*Esquema, len(archivos))
	for _, archivo := range archivos {
		crudo, err := fs.ReadFile(fsys, archivo)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(crudo))
		if err != nil {
			return nil, fmt.Errorf("esquema: %s no es JSON: %w", archivo, err)
		}

		// Una URL propia por archivo. Sin esquema absoluto, el compilador
		// intenta resolverla como ruta del sistema de archivos del proceso.
		url := "mem://esquemas/" + archivo
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("esquema: %s: %w", archivo, err)
		}
		s, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("esquema: %s no compila: %w", archivo, err)
		}
		out[strings.TrimSuffix(path.Base(archivo), ".json")] = &Esquema{s: s}
	}
	return out, nil
}

func soloStrings(validar func(string) error) func(any) error {
	return func(v any) error {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		return validar(s)
	}
}

// Validar devuelve TODOS los problemas de valor juntos, cada uno con su campo
// armado desde base: `value.name`, `blocks[2].props.items[0].title`. El
// formulario necesita marcar cada campo mal puesto de una vez; devolver solo el
// primero obliga a guardar diez veces para descubrir diez errores.
//
// El valor pasa por JSON antes de validarse. El validador espera los valores
// tal como los deja su propio decodificador (numeros como json.Number); darle
// lo que dejo encoding/json funciona hoy y deja de funcionar el dia que alguien
// construya el valor a mano con un int.
func (e *Esquema) Validar(valor any, base string) []httpx.FieldIssue {
	crudo, err := json.Marshal(valor)
	if err != nil {
		return uno(base, "type", "No se pudo leer como JSON")
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(crudo))
	if err != nil {
		return uno(base, "type", "No se pudo leer como JSON")
	}

	err = e.s.Validate(v)
	if err == nil {
		return nil
	}
	verr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return uno(base, "invalid", "No cumple su esquema")
	}

	var issues []httpx.FieldIssue
	for _, hoja := range hojas(verr) {
		issues = append(issues, traducir(hoja, base)...)
	}
	return issues
}

// Campo es un valor encontrado dentro de otro, con la ruta con que lo nombraria
// un error.
type Campo struct {
	Ruta  string
	Valor string
}

// ConFormato recorre valor siguiendo el esquema y devuelve los strings cuyo
// esquema declara ese `format`. Sirve para lo que el esquema no puede
// comprobar solo: que un `media-id` apunte a una imagen que existe.
//
// Se llama con un valor que ya paso Validar; lo que no casa con el esquema
// simplemente no se recorre.
func (e *Esquema) ConFormato(valor any, formato, base string) []Campo {
	var out []Campo
	recorrer(e.s, valor, formato, base, &out)
	return out
}

func recorrer(s *jsonschema.Schema, v any, formato, ruta string, out *[]Campo) {
	if s == nil {
		return
	}
	if s.Ref != nil {
		recorrer(s.Ref, v, formato, ruta, out)
	}
	if s.Format != nil && s.Format.Name == formato {
		if str, ok := v.(string); ok {
			*out = append(*out, Campo{Ruta: ruta, Valor: str})
		}
	}
	switch val := v.(type) {
	case map[string]any:
		for clave, sub := range s.Properties {
			if hijo, ok := val[clave]; ok {
				recorrer(sub, hijo, formato, ruta+"."+clave, out)
			}
		}
	case []any:
		for i, hijo := range val {
			recorrer(s.Items2020, hijo, formato, ruta+"["+strconv.Itoa(i)+"]", out)
		}
	}
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
			issues = append(issues, Obligatorio(campo+"."+falta))
		}
		return issues

	case *kind.AdditionalProperties:
		issues := make([]httpx.FieldIssue, 0, len(k.Properties))
		for _, sobra := range k.Properties {
			issues = append(issues, httpx.FieldIssue{
				Field: campo + "." + sobra, Code: "unknown",
				Message: "Este campo no se acepta aqui",
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
	case *kind.Pattern, *kind.Format:
		return uno(campo, "format", "No tiene el formato que acepta este campo")
	case *kind.Enum:
		return uno(campo, "oneof", "No es uno de los valores que acepta este campo")
	default:
		// Un tipo de error sin mensaje propio no se queda sin campo: el
		// formulario sigue sabiendo que resaltar, aunque el texto sea generico.
		return uno(campo, "invalid", "No cumple su esquema")
	}
}

// ruta arma `blocks[0].props.items[2].title` a partir de la ubicacion que da
// el validador, que viene partida en trozos y sin distinguir indices de claves.
// Un trozo numerico se escribe como indice: los esquemas del starter no usan
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

// Obligatorio es el problema de un campo que falta, con el mismo texto en todo
// el proyecto.
func Obligatorio(campo string) httpx.FieldIssue {
	return httpx.FieldIssue{Field: campo, Code: "required", Message: "Este campo es obligatorio"}
}

func uno(campo, code, mensaje string) []httpx.FieldIssue {
	return []httpx.FieldIssue{{Field: campo, Code: code, Message: mensaje}}
}
