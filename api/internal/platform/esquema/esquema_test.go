package esquema

import (
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// soloUno es un formato de prueba: acepta "uno" y nada mas.
var soloUno = Formato{Nombre: "solo-uno", Validar: func(s string) error {
	if s != "uno" {
		return errors.New("no es uno")
	}
	return nil
}}

const marca = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "minLength": 1, "maxLength": 5},
    "logo": {"type": "string", "format": "solo-uno"},
    "links": {
      "type": "array",
      "items": {"$ref": "#/$defs/enlace"}
    }
  },
  "$defs": {
    "enlace": {
      "type": "object",
      "properties": {"icono": {"type": "string", "format": "solo-uno"}}
    }
  }
}`

func cargar(t *testing.T, archivos map[string]string, formatos ...Formato) map[string]*Esquema {
	t.Helper()
	fsys := fstest.MapFS{}
	for nombre, contenido := range archivos {
		fsys["esquemas/"+nombre] = &fstest.MapFile{Data: []byte(contenido)}
	}
	out, err := Cargar(fsys, "esquemas/*.json", formatos...)
	if err != nil {
		t.Fatalf("Cargar: %v", err)
	}
	return out
}

func campos(issues []httpx.FieldIssue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Field+":"+i.Code)
	}
	slices.Sort(out)
	return out
}

func TestCadaArchivoSeGuardaPorSuNombre(t *testing.T) {
	es := cargar(t, map[string]string{"site.brand.json": marca, "otro.json": `{"type":"string"}`}, soloUno)
	for _, nombre := range []string{"site.brand", "otro"} {
		if es[nombre] == nil {
			t.Errorf("falta %q; hay %v", nombre, es)
		}
	}
}

func TestLosErroresSalenTodosJuntosConSuRuta(t *testing.T) {
	e := cargar(t, map[string]string{"m.json": marca}, soloUno)["m"]

	got := campos(e.Validar(map[string]any{"name": "demasiado largo", "sobra": 1, "logo": "dos"}, "value"))
	want := []string{"value.logo:format", "value.name:max", "value.sobra:unknown"}
	if !slices.Equal(got, want) {
		t.Errorf("= %v, se esperaba %v", got, want)
	}

	if got := campos(e.Validar(map[string]any{}, "value")); !slices.Equal(got, []string{"value.name:required"}) {
		t.Errorf("sin name = %v", got)
	}
	if issues := e.Validar(map[string]any{"name": "ok", "logo": "uno"}, "value"); issues != nil {
		t.Errorf("un valor valido dio %v", issues)
	}
}

// Sin AssertFormat, `format` es una anotacion y cualquier string pasa. Es el
// tipo de agujero que no avisa.
func TestUnFormatoPropioSeExigeTambienDentroDeUnRef(t *testing.T) {
	e := cargar(t, map[string]string{"m.json": marca}, soloUno)["m"]

	got := campos(e.Validar(map[string]any{"name": "ok", "links": []any{map[string]any{"icono": "dos"}}}, "value"))
	if !slices.Equal(got, []string{"value.links[0].icono:format"}) {
		t.Errorf("= %v", got)
	}
}

func TestConFormatoEncuentraLosValoresPorElEsquemaYNoPorElNombre(t *testing.T) {
	e := cargar(t, map[string]string{"m.json": marca}, soloUno)["m"]
	valor := map[string]any{
		"name":  "ok",
		"logo":  "uno",
		"links": []any{map[string]any{"icono": "uno"}, map[string]any{}, map[string]any{"icono": "uno"}},
	}

	got := e.ConFormato(valor, "solo-uno", "value")
	rutas := make([]string, 0, len(got))
	for _, c := range got {
		rutas = append(rutas, c.Ruta+"="+c.Valor)
	}
	slices.Sort(rutas)
	want := []string{"value.links[0].icono=uno", "value.links[2].icono=uno", "value.logo=uno"}
	if !slices.Equal(rutas, want) {
		t.Errorf("= %v, se esperaba %v", rutas, want)
	}
	if got := e.ConFormato(map[string]any{"name": "ok"}, "solo-uno", "value"); len(got) != 0 {
		t.Errorf("sin logo encontro %v", got)
	}
}

func TestUnEsquemaMalEscritoNoCarga(t *testing.T) {
	for nombre, contenido := range map[string]string{
		"no es JSON":            `{"type": "object",`,
		"no es un esquema":      `{"type": "cosa"}`,
		"patron que no compila": `{"type":"string","pattern":"("}`,
	} {
		t.Run(nombre, func(t *testing.T) {
			fsys := fstest.MapFS{"esquemas/roto.json": {Data: []byte(contenido)}}
			if _, err := Cargar(fsys, "esquemas/*.json"); err == nil {
				t.Error("cargo un esquema roto")
			}
		})
	}
}
