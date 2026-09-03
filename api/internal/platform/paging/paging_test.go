package paging_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// spec de prueba con la misma forma que la de un recurso real: dos campos
// ordenables, dos filtrables y un desempate.
func spec() *paging.Spec {
	return paging.NewSpec("key").
		SortBy("key", "key").
		SortBy("updatedAt", "updated_at").
		FilterBy("isPublic", "is_public", paging.Bool).
		FilterBy("key", "key", paging.Text)
}

func parse(t *testing.T, query string) (paging.Params, *httpx.Problem) {
	t.Helper()
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("query de prueba mal escrita: %v", err)
	}
	return spec().Parse(q)
}

func parseOK(t *testing.T, query string) paging.Params {
	t.Helper()
	p, prob := parse(t, query)
	if prob != nil {
		t.Fatalf("no se esperaba problema para %q: %+v", query, prob)
	}
	return p
}

func esperaProblema(t *testing.T, query, campo string) *httpx.Problem {
	t.Helper()
	_, prob := parse(t, query)
	if prob == nil {
		t.Fatalf("%q tenia que ser rechazada", query)
	}
	if prob.Status != http.StatusBadRequest || prob.Code != httpx.CodeValidationFailed {
		t.Fatalf("status/code = %d/%s; se esperaba 400 VALIDATION_FAILED", prob.Status, prob.Code)
	}
	if len(prob.Errors) != 1 || prob.Errors[0].Field != campo {
		t.Fatalf("errors = %+v; tiene que nombrar el campo %q", prob.Errors, campo)
	}
	return prob
}

func TestSinParametrosUsaLosValoresPorOmision(t *testing.T) {
	p := parseOK(t, "")

	if p.Number != 0 || p.Size != paging.DefaultSize {
		t.Errorf("number/size = %d/%d, se esperaba 0/%d", p.Number, p.Size, paging.DefaultSize)
	}
	if got := p.OrderBy(); got != "order by key asc" {
		t.Errorf("orderBy = %q", got)
	}
}

// El criterio de la HU y del checklist: el tope es duro y NO es un error.
func TestUnTamanoAbsurdoSeTopaYSeReportaElEfectivo(t *testing.T) {
	p := parseOK(t, "size=1000000")

	if p.Size != paging.MaxSize {
		t.Fatalf("size = %d; el tope tiene que ser duro en %d", p.Size, paging.MaxSize)
	}
	if p.Limit() != paging.MaxSize {
		t.Errorf("limit = %d; la consulta tiene que llevar el tamano topado", p.Limit())
	}

	pagina := paging.NewPage([]string{"a"}, p, 1)
	if pagina.Page.Size != paging.MaxSize {
		t.Errorf("page.size = %d; tiene que reportar el tamano EFECTIVO, no el pedido", pagina.Page.Size)
	}
}

func TestUnTamanoInvalidoEs400(t *testing.T) {
	for _, q := range []string{"size=0", "size=-1", "size=abc", "size=1.5"} {
		esperaProblema(t, q, "size")
	}
}

func TestUnaPaginaInvalidaEs400(t *testing.T) {
	for _, q := range []string{"page=-1", "page=x"} {
		esperaProblema(t, q, "page")
	}
}

// El otro criterio del checklist: un sort fuera de la lista blanca es 400, no
// un error de SQL filtrado como 500.
func TestUnSortFueraDeLaListaBlancaEs400YDiceCualesValen(t *testing.T) {
	prob := esperaProblema(t, "sort=passwordHash,desc", "sort")

	if !strings.Contains(prob.Errors[0].Message, "updatedAt") {
		t.Errorf("el mensaje tiene que decir que campos SI valen: %q", prob.Errors[0].Message)
	}
}

func TestUnaDireccionInvalidaEs400(t *testing.T) {
	esperaProblema(t, "sort=key,arriba", "sort")
}

func TestElOrdenSeTraduceAColumnaReal(t *testing.T) {
	p := parseOK(t, "sort=updatedAt,desc")

	if got := p.OrderBy(); got != "order by updated_at desc, key asc" {
		t.Errorf("orderBy = %q; el campo del contrato tiene que salir como columna", got)
	}
}

func TestElOrdenAcumulaVariosCriterios(t *testing.T) {
	p := parseOK(t, "sort=updatedAt,desc&sort=key,asc")

	if got := p.OrderBy(); got != "order by updated_at desc, key asc" {
		t.Errorf("orderBy = %q", got)
	}
}

// El desempate estable es la diferencia entre paginar y perder filas.
func TestTodoOrdenTerminaEnElDesempate(t *testing.T) {
	for _, q := range []string{"", "sort=updatedAt,desc", "sort=updatedAt,asc"} {
		p := parseOK(t, q)
		if !strings.HasSuffix(p.OrderBy(), "key asc") {
			t.Errorf("orderBy(%q) = %q; tiene que terminar en el desempate", q, p.OrderBy())
		}
	}
}

// Y no se repite cuando el cliente ya ordeno por esa columna.
func TestElDesempateNoSeDuplica(t *testing.T) {
	p := parseOK(t, "sort=key,desc")

	if got := p.OrderBy(); got != "order by key desc" {
		t.Errorf("orderBy = %q; ordenar por la columna de desempate no debe repetirla", got)
	}
	if strings.Count(p.OrderBy(), "key") != 1 {
		t.Errorf("orderBy = %q; la columna aparece dos veces", p.OrderBy())
	}
}

func TestUnFiltroNoDeclaradoSeRechaza(t *testing.T) {
	esperaProblema(t, "createdBy=otro", "createdBy")
}

func TestUnFiltroBooleanoExigeUnBooleano(t *testing.T) {
	esperaProblema(t, "isPublic=quiza", "isPublic")
}

func TestUnFiltroRepetidoSeRechaza(t *testing.T) {
	esperaProblema(t, "isPublic=true&isPublic=false", "isPublic")
}

func TestElFiltroSaleComoParametro(t *testing.T) {
	p := parseOK(t, "isPublic=true")

	clausula, args := p.Where(1)
	if clausula != "where is_public = $1" {
		t.Errorf("where = %q", clausula)
	}
	if len(args) != 1 || args[0] != true {
		t.Errorf("args = %#v; el valor tiene que viajar convertido, no como texto", args)
	}
}

func TestVariosFiltrosNumeranLosMarcadoresDesdeDondeSeLesDice(t *testing.T) {
	p := parseOK(t, "isPublic=true&key=site.brand")

	clausula, args := p.Where(3)
	if clausula != "where is_public = $3 and key = $4" {
		t.Errorf("where = %q", clausula)
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v", args)
	}
}

func TestSinFiltrosNoHayClausula(t *testing.T) {
	clausula, args := parseOK(t, "").Where(1)
	if clausula != "" || args != nil {
		t.Errorf("where = %q, args = %#v; sin filtros no se escribe nada", clausula, args)
	}
}

// La invariante del paquete: ningun fragmento de SQL del cliente llega a la
// consulta. Ni por el orden, que se rechaza, ni por el filtro, que se
// parametriza.
func TestNingunFragmentoDelClienteLlegaAlSql(t *testing.T) {
	veneno := "'; drop table settings; --"

	if _, prob := parse(t, "sort="+url.QueryEscape(veneno)); prob == nil {
		t.Fatal("un sort con SQL tiene que rechazarse")
	}

	p := parseOK(t, "key="+url.QueryEscape(veneno))
	clausula, args := p.Where(1)
	if strings.Contains(clausula, "drop") {
		t.Errorf("la clausula interpolo el valor: %q", clausula)
	}
	if clausula != "where key = $1" {
		t.Errorf("where = %q", clausula)
	}
	if len(args) != 1 || args[0] != veneno {
		t.Errorf("args = %#v; el valor tiene que viajar como parametro, intacto", args)
	}
}

func TestElOffsetSaleDeLaPaginaYDelTamano(t *testing.T) {
	p := parseOK(t, "page=3&size=25")

	if p.Offset() != 75 || p.Limit() != 25 {
		t.Errorf("offset/limit = %d/%d, se esperaba 75/25", p.Offset(), p.Limit())
	}
}

func TestElEnvoltorioNoDevuelveNuncaNull(t *testing.T) {
	pagina := paging.NewPage[string](nil, parseOK(t, ""), 0)

	crudo, err := json.Marshal(pagina)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(crudo), `"content":[]`) {
		t.Errorf("cuerpo = %s; una coleccion vacia sale como [], nunca como null", crudo)
	}
}

func TestElTotalDePaginasRedondeaHaciaArriba(t *testing.T) {
	casos := []struct {
		total    int64
		size     string
		esperado int
	}{
		{0, "size=20", 0},
		{1, "size=20", 1},
		{20, "size=20", 1},
		{21, "size=20", 2},
		{137, "size=20", 7},
	}

	for _, c := range casos {
		p := parseOK(t, c.size)
		got := paging.NewPage([]string{}, p, c.total).Page.TotalPages
		if got != c.esperado {
			t.Errorf("total=%d size=%s: totalPages = %d, se esperaba %d", c.total, c.size, got, c.esperado)
		}
	}
}

// Una pagina mas alla del final es una respuesta valida y vacia, no un error:
// el cliente que llego al final no merece un 400.
func TestUnaPaginaFueraDeRangoNoEsUnError(t *testing.T) {
	p := parseOK(t, "page=99")

	pagina := paging.NewPage([]string{}, p, 3)
	if pagina.Page.Number != 99 || pagina.Page.TotalElements != 3 {
		t.Errorf("meta = %+v", pagina.Page)
	}
	if len(pagina.Content) != 0 {
		t.Errorf("content = %+v", pagina.Content)
	}
}

// El desempate no puede ser opcional, asi que no se puede construir un Spec sin
// el. Si se pudiera, alguien lo olvidaria y las filas empatadas se perderian.
func TestUnSpecSinDesempateNoSePuedeConstruir(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("se esperaba un panic: un Spec sin desempate es un bug de programacion")
		}
	}()

	paging.NewSpec("")
}
