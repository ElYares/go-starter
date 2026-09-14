package content

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestElCatalogoCompilaYTraeLosTiposDelStarter(t *testing.T) {
	cat, err := cargarCatalogo(bloquesFS)
	if err != nil {
		t.Fatalf("el catalogo no compila: %v", err)
	}
	if got := cat.tipos(); !slices.Equal(got, []string{"features", "hero", "texto"}) {
		t.Errorf("tipos = %v", got)
	}
}

// Un esquema roto tiene que impedir arrancar, no convertirse en un 500 la
// primera vez que alguien guarda ese bloque.
func TestUnEsquemaMalEscritoImpideArmarElCatalogo(t *testing.T) {
	casos := map[string]string{
		"no es JSON":            `{"type": "object",`,
		"no es un esquema":      `{"type": "cosa"}`,
		"patron que no compila": `{"type":"string","pattern":"("}`,
	}
	for nombre, contenido := range casos {
		t.Run(nombre, func(t *testing.T) {
			fsys := fstest.MapFS{"bloques/roto.json": {Data: []byte(contenido)}}
			if _, err := cargarCatalogo(fsys); err == nil {
				t.Error("se armo el catalogo con un esquema roto")
			}
		})
	}
}

func TestUnCatalogoVacioNoArranca(t *testing.T) {
	if _, err := cargarCatalogo(fstest.MapFS{}); err == nil {
		t.Error("se armo un catalogo sin tipos")
	}
}

// La pagina que siembra la migracion tiene que pasar su propia validacion. Si
// no, el primer guardado de quien la edite sin tocar nada rebota con un 400 que
// no entiende, porque el error lo puso la semilla y no el.
//
// Se lee del .sql y no de la base: en la base la pagina ya puede estar editada.
func TestLaPaginaSembradaCumpleElCatalogo(t *testing.T) {
	sql, err := os.ReadFile("migrations/0001_contenido.sql")
	if err != nil {
		t.Fatal(err)
	}

	m := regexp.MustCompile(`(?s)'(\[\s*\{"id".*?\])'`).FindSubmatch(sql)
	if m == nil {
		t.Fatal("no se encontraron los bloques sembrados en la migracion")
	}

	var bloques []Bloque
	if err := json.Unmarshal(m[1], &bloques); err != nil {
		t.Fatalf("los bloques sembrados no son JSON: %v", err)
	}

	cat, _ := cargarCatalogo(bloquesFS)
	if len(bloques) != len(cat.tipos()) {
		t.Errorf("la semilla tiene %d bloques y el catalogo %d tipos: tiene que mostrar uno de cada", len(bloques), len(cat.tipos()))
	}
	if issues := cat.validar(bloques); len(issues) > 0 {
		t.Errorf("la semilla no cumple el catalogo: %+v", issues)
	}
}

// El validador distingue un numero entero del que construyo encoding/json y
// del que construiria alguien a mano. Pasar por JSON antes de validar hace que
// den lo mismo.
func TestLasPropsConstruidasEnGoSeValidanIgualQueLasDecodificadas(t *testing.T) {
	cat, _ := cargarCatalogo(bloquesFS)

	aMano := []Bloque{{Id: "b1", Type: "hero", Props: map[string]interface{}{"title": 7}}}
	decodificadas := []Bloque{}
	_ = json.Unmarshal([]byte(`[{"id":"b1","type":"hero","props":{"title":7}}]`), &decodificadas)

	a, d := cat.validar(aMano), cat.validar(decodificadas)
	if len(a) != 1 || len(d) != 1 || a[0] != d[0] {
		t.Errorf("a mano = %+v, decodificadas = %+v", a, d)
	}
}

// La unica otra unicidad del modulo —el numero de version— no es culpa del
// cliente. Reportarla como "slug ocupado" mandaria a buscar el error donde no
// esta. No se puede provocar desde fuera (el bloqueo de la pagina lo impide),
// asi que se prueba la traduccion directamente.
func TestSoloLaUnicidadDelSlugSeTraduceASlugOcupado(t *testing.T) {
	slug := &pgconn.PgError{Code: "23505", ConstraintName: "pages_slug_key"}
	if err := traducirPg(slug); !errors.Is(err, errSlugOcupado) {
		t.Errorf("slug repetido = %v", err)
	}

	numero := &pgconn.PgError{Code: "23505", ConstraintName: "page_versions_numero_key"}
	if err := traducirPg(numero); errors.Is(err, errSlugOcupado) {
		t.Error("un numero de version repetido salio como slug ocupado")
	}
}
