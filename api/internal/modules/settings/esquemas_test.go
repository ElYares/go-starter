package settings

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Las cuatro claves del starter tienen esquema. Es la lista que la landing y el
// dashboard dan por hecha.
func TestLasClavesDelStarterTienenEsquema(t *testing.T) {
	es, err := cargarEsquemas()
	if err != nil {
		t.Fatalf("los esquemas no compilan: %v", err)
	}
	claves := make([]string, 0, len(es))
	for k := range es {
		claves = append(claves, k)
	}
	slices.Sort(claves)
	if want := []string{"site.brand", "site.footer", "site.nav", "site.theme"}; !slices.Equal(claves, want) {
		t.Errorf("claves = %v, se esperaba %v", claves, want)
	}
}

// Del esquema sale el formulario del dashboard (Decision 022), y una propiedad
// sin `title` sale como un campo que se llama "logo" o "href".
func TestCadaPropiedadTieneTitulo(t *testing.T) {
	archivos, err := fs.Glob(esquemasFS, "esquemas/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, archivo := range archivos {
		crudo, _ := fs.ReadFile(esquemasFS, archivo)
		var doc map[string]any
		if err := json.Unmarshal(crudo, &doc); err != nil {
			t.Fatalf("%s: %v", archivo, err)
		}
		sinTitulo(t, archivo, "", doc)
	}
}

func sinTitulo(t *testing.T, archivo, ruta string, nodo map[string]any) {
	t.Helper()
	for _, clave := range []string{"properties", "$defs"} {
		hijos, _ := nodo[clave].(map[string]any)
		for nombre, h := range hijos {
			hijo, _ := h.(map[string]any)
			if clave == "properties" {
				if _, ok := hijo["title"]; !ok {
					t.Errorf("%s: %s.%s no tiene title", archivo, ruta, nombre)
				}
			}
			sinTitulo(t, archivo, ruta+"."+nombre, hijo)
		}
	}
	if items, ok := nodo["items"].(map[string]any); ok {
		sinTitulo(t, archivo, ruta+"[]", items)
	}
}

// Lo que siembran las migraciones tiene que pasar su propia validacion. Si no,
// el primer guardado de quien no toco nada rebota con un 400 que no entiende.
//
// Se lee de los .sql y no de la base: en la base ya puede estar editado.
func TestLoSembradoCumpleSuEsquema(t *testing.T) {
	es, err := cargarEsquemas()
	if err != nil {
		t.Fatal(err)
	}
	fila := regexp.MustCompile(`\('(site\.[a-z]+)',\s*'(.*?)'`)

	vistas := map[string]bool{}
	for _, archivo := range []string{"migrations/0001_settings.sql", "migrations/0002_pie.sql"} {
		sql, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range fila.FindAllSubmatch(sql, -1) {
			clave := string(m[1])
			var valor any
			if err := json.Unmarshal(m[2], &valor); err != nil {
				t.Fatalf("%s: el valor de %s no es JSON: %v", archivo, clave, err)
			}
			e, ok := es[clave]
			if !ok {
				t.Errorf("%s siembra %s, que no tiene esquema", archivo, clave)
				continue
			}
			if issues := e.Validar(valor, "value"); issues != nil {
				t.Errorf("%s: %s no cumple su esquema: %+v", archivo, clave, issues)
			}
			vistas[clave] = true
		}
	}
	for _, clave := range []string{"site.brand", "site.nav", "site.theme", "site.footer"} {
		if !vistas[clave] {
			t.Errorf("ninguna migracion siembra %s", clave)
		}
	}
}

// guardar es un PUT sobre una clave con un valor, y devuelve el estado, los
// campos del 400 si lo hubo, y si llego a escribirse.
func guardar(t *testing.T, medios Medios, clave, valor string) (int, []string, bool) {
	t.Helper()
	repo := &repoFalso{items: []Setting{unSetting(clave)}}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/"+clave, strings.NewReader(`{"value":`+valor+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"7"`)
	rec := httptest.NewRecorder()
	servidorCon(repo, medios, escritor()).ServeHTTP(rec, req)

	var campos []string
	if rec.Code == http.StatusBadRequest {
		for _, e := range problema(t, rec).Errors {
			campos = append(campos, e.Field+":"+e.Code)
		}
		slices.Sort(campos)
	}
	return rec.Code, campos, len(repo.sello.Columns) > 0
}

func TestSinNombreLaMarcaEs400EnValueNameYNoSeEscribe(t *testing.T) {
	estado, campos, escrito := guardar(t, mediosFalsos{}, "site.brand", `{"tagline":"sin nombre"}`)

	if estado != http.StatusBadRequest || !slices.Equal(campos, []string{"value.name:required"}) {
		t.Fatalf("estado = %d, campos = %v", estado, campos)
	}
	if escrito {
		t.Error("un valor invalido llego al repositorio: la version habria subido")
	}
}

// Un enlace `javascript:` en el menu es un XSS en cada pagina de la landing. Y
// `//otro.com` empieza por `/` pero lleva a otro sitio.
func TestUnEnlaceSoloPuedeSerUnaRutaDelSitioOHttps(t *testing.T) {
	malos := []string{"javascript:alert(1)", "JAVASCRIPT:alert(1)", "//otro.com", "http://otro.com", "data:text/html,x", "precios"}
	buenos := []string{"/", "/precios", "/precios#planes", "https://otro.com"}

	for _, href := range malos {
		nav := `[{"label":"Malo","href":"` + href + `"}]`
		if estado, campos, _ := guardar(t, mediosFalsos{}, "site.nav", nav); estado != http.StatusBadRequest ||
			!slices.Equal(campos, []string{"value[0].href:format"}) {
			t.Errorf("nav %q: estado = %d, campos = %v", href, estado, campos)
		}
		pie := `{"links":[{"label":"Malo","href":"` + href + `"}]}`
		if estado, campos, _ := guardar(t, mediosFalsos{}, "site.footer", pie); estado != http.StatusBadRequest ||
			!slices.Equal(campos, []string{"value.links[0].href:format"}) {
			t.Errorf("pie %q: estado = %d, campos = %v", href, estado, campos)
		}
	}
	for _, href := range buenos {
		nav := `[{"label":"Bueno","href":"` + href + `"}]`
		if estado, campos, _ := guardar(t, mediosFalsos{}, "site.nav", nav); estado != http.StatusOK {
			t.Errorf("nav %q: estado = %d, campos = %v", href, estado, campos)
		}
	}
}

func TestElLogoTieneQueSerUnaImagenQueExiste(t *testing.T) {
	existe := uuid.New()
	medios := mediosFalsos{existe: true}

	if estado, campos, _ := guardar(t, medios, "site.brand", `{"name":"X","logo":"`+existe.String()+`"}`); estado != http.StatusOK {
		t.Errorf("logo existente: estado = %d, campos = %v", estado, campos)
	}

	estado, campos, escrito := guardar(t, medios, "site.brand", `{"name":"X","logo":"`+uuid.NewString()+`"}`)
	if estado != http.StatusBadRequest || !slices.Equal(campos, []string{"value.logo:unknown"}) {
		t.Errorf("logo que no existe: estado = %d, campos = %v", estado, campos)
	}
	if escrito {
		t.Error("un logo que no existe llego al repositorio")
	}

	if estado, campos, _ := guardar(t, medios, "site.brand", `{"name":"X","logo":"logo.png"}`); estado != http.StatusBadRequest ||
		!slices.Equal(campos, []string{"value.logo:format"}) {
		t.Errorf("logo que no es uuid: estado = %d, campos = %v", estado, campos)
	}
}

// Si los errores del esquema ya son un 400, no se pregunta nada a media: el
// cliente arregla primero lo que ya se sabe que esta mal.
func TestConOtrosErroresNoSePreguntaPorElLogo(t *testing.T) {
	espia := &mediosQueCuentan{}
	guardar(t, espia, "site.brand", `{"logo":"`+uuid.NewString()+`"}`)
	if espia.llamadas != 0 {
		t.Errorf("se pregunto %d veces por el logo con el nombre faltando", espia.llamadas)
	}
}

type mediosQueCuentan struct{ llamadas int }

func (m *mediosQueCuentan) Existe(context.Context, uuid.UUID) (bool, error) {
	m.llamadas++
	return true, nil
}

type mediosRotos struct{}

func (mediosRotos) Existe(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("la base no responde")
}

// Que media falle no es culpa del cliente: es un 500, no un 400 que le diga que
// su logo no existe.
func TestSiMediaFallaEs500YNoUn400(t *testing.T) {
	if estado, campos, _ := guardar(t, mediosRotos{}, "site.brand", `{"name":"X","logo":"`+uuid.NewString()+`"}`); estado != http.StatusInternalServerError {
		t.Errorf("estado = %d, campos = %v", estado, campos)
	}
}

func TestElAcentoTieneQueSerUnColorHexadecimal(t *testing.T) {
	for _, malo := range []string{"azul", "#2f6df", "#2f6df6ff", "2f6df6"} {
		if estado, campos, _ := guardar(t, mediosFalsos{}, "site.theme", `{"accent":"`+malo+`"}`); estado != http.StatusBadRequest ||
			!slices.Equal(campos, []string{"value.accent:format"}) {
			t.Errorf("%q: estado = %d, campos = %v", malo, estado, campos)
		}
	}
	if estado, _, _ := guardar(t, mediosFalsos{}, "site.theme", `{"accent":"#2F6DF6"}`); estado != http.StatusOK {
		t.Errorf("mayusculas: estado = %d", estado)
	}
}

// Toda clave la declara el codigo con su esquema (Decision 024). Crear una
// suelta por la API dejaba que la landing leyera cualquier cosa.
func TestUnaClaveSinEsquemaEs400YNoSeEscribe(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings",
		cuerpo: `{"key":"site.inventada","value":{"a":1}}`,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	p := problema(t, rec)
	if len(p.Errors) != 1 || p.Errors[0].Field != "key" || p.Errors[0].Code != "unknown" {
		t.Errorf("errors = %+v", p.Errors)
	}
	if !strings.Contains(p.Errors[0].Message, "site.brand") {
		t.Errorf("el mensaje no dice cuales hay: %q", p.Errors[0].Message)
	}
	if len(repo.sello.Columns) > 0 {
		t.Error("se escribio una clave sin esquema")
	}

	put := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.inventada",
		cuerpo: `{"value":{"a":1}}`, ifMatch: `"1"`,
	})
	if put.Code != http.StatusBadRequest {
		t.Errorf("PUT de una clave sin esquema: estado = %d", put.Code)
	}
}

func ptr(b bool) *bool { return &b }

// Un PUT sin isPublic conserva la visibilidad. Antes, ausente era `false`, y un
// formulario que solo editaba el valor escondia la clave de la landing en cada
// guardado.
func TestUnPutSinIsPublicConservaLaVisibilidad(t *testing.T) {
	casos := map[string]*bool{
		`{"value":{"name":"X"}}`:                  nil,
		`{"value":{"name":"X"},"isPublic":true}`:  ptr(true),
		`{"value":{"name":"X"},"isPublic":false}`: ptr(false),
	}
	for cuerpo, esperado := range casos {
		repo := &repoFalso{items: []Setting{unSetting("site.brand")}}
		rec := llamar(t, repo, escritor(), peticion{
			metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand", cuerpo: cuerpo, ifMatch: `"7"`,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: estado = %d: %s", cuerpo, rec.Code, rec.Body.String())
		}
		switch {
		case esperado == nil && repo.publica != nil:
			t.Errorf("%s: llego isPublic=%v al repositorio; ausente tiene que conservar", cuerpo, *repo.publica)
		case esperado != nil && (repo.publica == nil || *repo.publica != *esperado):
			t.Errorf("%s: llego %v, se esperaba %v", cuerpo, repo.publica, *esperado)
		}
	}
}
