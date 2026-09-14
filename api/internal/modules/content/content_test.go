package content

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

const idDeAna = "3f1c9b2e-7d5a-4c81-9e0f-2a6b8c4d1e33"

var (
	idDePagina  = uuid.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b")
	idDeVersion = uuid.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c")
)

// repoFalso reemplaza a Postgres. Anota lo que el service le pidio, para
// afirmar sobre eso: el limite, el orden, la version del If-Match y, sobre
// todo, si llego a escribir algo.
type repoFalso struct {
	pagina_   Pagina
	resumen   []PaginaResumen
	total     int64
	err       error
	params    paging.Params
	version   int
	escrito   *borrador
	publicada uuid.UUID
}

func (r *repoFalso) pagina(_ context.Context, p paging.Params) ([]PaginaResumen, int64, error) {
	r.params = p
	return r.resumen, r.total, r.err
}

func (r *repoFalso) versiones(_ context.Context, _ uuid.UUID, p paging.Params) ([]VersionResumen, int64, error) {
	r.params = p
	return nil, 0, r.err
}

func (r *repoFalso) obtener(context.Context, uuid.UUID) (Pagina, error) { return r.pagina_, r.err }

func (r *repoFalso) publica(context.Context, string) (PaginaPublica, error) {
	return PaginaPublica{Slug: r.pagina_.Slug, Title: r.pagina_.Title, Blocks: r.pagina_.Blocks}, r.err
}

func (r *repoFalso) crear(_ context.Context, b borrador) (Pagina, error) {
	r.escrito = &b
	if r.err != nil {
		return Pagina{}, r.err
	}
	return Pagina{Id: idDePagina, Slug: b.Slug, Title: b.Title, Blocks: b.Blocks, Version: 1, DraftNumber: 1}, nil
}

func (r *repoFalso) guardar(_ context.Context, _ uuid.UUID, version int, b borrador) (Pagina, error) {
	r.version = version
	r.escrito = &b
	if r.err != nil {
		return Pagina{}, r.err
	}
	return Pagina{Id: idDePagina, Slug: b.Slug, Title: b.Title, Blocks: b.Blocks, Version: version + 1}, nil
}

func (r *repoFalso) publicar(_ context.Context, _ uuid.UUID, v uuid.UUID) (Pagina, error) {
	r.publicada = v
	return r.pagina_, r.err
}

func (r *repoFalso) borrar(context.Context, uuid.UUID) error { return r.err }

func servidor(t *testing.T, repo repositorio, actor *rbac.Actor) http.Handler {
	t.Helper()
	cat, err := cargarCatalogo(bloquesFS)
	if err != nil {
		t.Fatalf("catalogo: %v", err)
	}

	m := &Module{svc: &Service{repo: repo, cat: cat}}
	r := httpx.NewRouter()
	m.Routes(r)

	inyectar := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if actor != nil {
				req = req.WithContext(rbac.WithActor(req.Context(), *actor))
			}
			next.ServeHTTP(w, req)
		})
	}
	return observ.Chain(r.Handler(), observ.TraceID, inyectar)
}

func con(permisos ...string) *rbac.Actor {
	return &rbac.Actor{ID: idDeAna, Permissions: permisos}
}

func lector() *rbac.Actor { return con("content.page.read") }

func editor() *rbac.Actor { return con("content.page.read", "content.page.write") }

type peticion struct {
	metodo  string
	ruta    string
	cuerpo  string
	ifMatch string
}

func llamar(t *testing.T, repo repositorio, actor *rbac.Actor, p peticion) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(p.metodo, p.ruta, strings.NewReader(p.cuerpo))
	if p.cuerpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.ifMatch != "" {
		req.Header.Set("If-Match", p.ifMatch)
	}
	rec := httptest.NewRecorder()
	servidor(t, repo, actor).ServeHTTP(rec, req)
	return rec
}

func problema(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("el cuerpo no es problem+json: %q", rec.Body.String())
	}
	if p.TraceID == "" {
		t.Error("el error salio sin traceId: no se puede encontrar en el log")
	}
	return p
}

// campos devuelve "campo:code" de cada problema, ordenados, para comparar sin
// depender del orden en que el validador los encontro.
func campos(p httpx.Problem) []string {
	out := make([]string, 0, len(p.Errors))
	for _, e := range p.Errors {
		out = append(out, e.Field+":"+e.Code)
	}
	slices.Sort(out)
	return out
}

func rutaDePagina(sufijo string) string { return "/api/v1/pages/" + idDePagina.String() + sufijo }

// cuerpo arma una pagina con los bloques dados. Los bloques van como JSON
// crudo: lo que se prueba es lo que manda un cliente, no un struct de Go.
func cuerpo(bloques string) string {
	return `{"slug":"nosotros","title":"Nosotros","blocks":` + bloques + `}`
}

const heroValido = `{"id":"b1","type":"hero","props":{"title":"Hola","cta":{"label":"Ver","href":"/precios"}}}`

// ---------------------------------------------------------------- coleccion

func TestLaColeccionSaleConElEnvoltorioYNuncaComoArray(t *testing.T) {
	repo := &repoFalso{resumen: []PaginaResumen{{Id: idDePagina, Slug: "inicio", Title: "Inicio"}}, total: 1}
	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/pages"})

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	var cuerpo map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("la coleccion no es un objeto: %s", rec.Body.String())
	}
	if _, ok := cuerpo["content"]; !ok {
		t.Error("falta content")
	}
	if _, ok := cuerpo["page"]; !ok {
		t.Error("falta page")
	}
}

func TestUnTamanoAbsurdoDevuelveElTopeYLaConsultaLoRespeta(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/pages?size=1000000"})

	// El contrato declara maximum: 100, asi que el enlace generado no lo
	// rechaza (no valida rangos) y paging lo recorta. Lo que importa es que la
	// consulta nunca pida mas.
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.params.Limit() != 100 {
		t.Errorf("limite pedido al repositorio = %d, se esperaba 100", repo.params.Limit())
	}
	var pagina PaginasPage
	_ = json.Unmarshal(rec.Body.Bytes(), &pagina)
	if pagina.Page.Size != 100 {
		t.Errorf("page.size = %d, tiene que reportar el tamano efectivo", pagina.Page.Size)
	}
}

func TestUnSortInvalidoEs400YNo500(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/pages?sort=blocks"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	problema(t, rec)
}

func TestSinOrdenPedidoLasPaginasSalenPorLaUltimaModificacion(t *testing.T) {
	repo := &repoFalso{}
	llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/pages?published=true"})

	if got := repo.params.OrderBy(); got != "order by updated_at desc, id asc" {
		t.Errorf("orden = %q", got)
	}
	if where, args := repo.params.Where(1); where != "where published = $1" || len(args) != 1 || args[0] != true {
		t.Errorf("filtro = %q %v", where, args)
	}
}

func TestLasVersionesSalenDeLaMasNuevaALaMasVieja(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: rutaDePagina("/versions")})

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if got := repo.params.OrderBy(); got != "order by number desc" {
		t.Errorf("orden = %q", got)
	}
}

func TestLasVersionesDeUnaPaginaQueNoExisteSon404(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errNoExiste}, lector(), peticion{metodo: http.MethodGet, ruta: rutaDePagina("/versions")})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------- permisos

func TestSinSesionEs401(t *testing.T) {
	rec := llamar(t, &repoFalso{}, nil, peticion{metodo: http.MethodGet, ruta: "/api/v1/pages"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	problema(t, rec)
}

func TestConSesionSinPermisoDeEscrituraEs403(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo("[]")})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.escrito != nil {
		t.Error("se escribio sin permiso")
	}
}

// Guardar y publicar son permisos distintos: quien edita puede no poder
// cambiar lo que ve el publico.
func TestQuienPuedeGuardarNoPuedePublicarSinSuPermiso(t *testing.T) {
	rec := llamar(t, &repoFalso{}, editor(), peticion{
		metodo: http.MethodPost, ruta: rutaDePagina("/publish"),
		cuerpo: `{"versionId":"` + idDeVersion.String() + `"}`,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLaPaginaPublicaNoPideSesion(t *testing.T) {
	repo := &repoFalso{pagina_: Pagina{Slug: "inicio", Title: "Inicio", Blocks: []Bloque{}}}
	rec := llamar(t, repo, nil, peticion{metodo: http.MethodGet, ruta: "/api/v1/public/pages/inicio"})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

// Criterio 2 de HU-009, del lado HTTP: sin version publicada el repositorio no
// devuelve fila, y eso sale como 404 y no como una pagina vacia.
func TestUnaPaginaSinPublicarEs404ParaElPublico(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errNoExiste}, nil, peticion{metodo: http.MethodGet, ruta: "/api/v1/public/pages/nosotros"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	problema(t, rec)
}

// Un id que no es uuid lo rechaza el enlace generado, como problem+json y
// nombrando el parametro. Con sesion: sin ella, el guard contesta 401 antes.
func TestUnIdQueNoEsUuidEs400NombrandoElParametro(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/pages/no-es-uuid"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "id" {
		t.Errorf("errores = %+v", p.Errors)
	}
}

// ---------------------------------------------------------------- If-Match

func TestLeerDevuelveElETagQueDespuesSePideEnIfMatch(t *testing.T) {
	repo := &repoFalso{pagina_: Pagina{Id: idDePagina, Version: 7, Blocks: []Bloque{}}}
	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: rutaDePagina("")})
	if got := rec.Header().Get("ETag"); got != `"7"` {
		t.Errorf("ETag = %q", got)
	}
}

func TestGuardarSinIfMatchEs400(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, editor(), peticion{metodo: http.MethodPut, ruta: rutaDePagina(""), cuerpo: cuerpo("[]")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.escrito != nil {
		t.Error("se guardo sin If-Match")
	}
}

func TestUnIfMatchViejoEs409(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errVersion}, editor(), peticion{
		metodo: http.MethodPut, ruta: rutaDePagina(""), cuerpo: cuerpo("[]"), ifMatch: `"3"`,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); p.Code != httpx.CodeConflict {
		t.Errorf("code = %q", p.Code)
	}
}

func TestLaVersionQueLlegaAlRepositorioEsLaDelIfMatchYElETagSube(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, editor(), peticion{
		metodo: http.MethodPut, ruta: rutaDePagina(""), cuerpo: cuerpo("[" + heroValido + "]"), ifMatch: `"7"`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.version != 7 {
		t.Errorf("version pedida = %d, se esperaba la del If-Match", repo.version)
	}
	if got := rec.Header().Get("ETag"); got != `"8"` {
		t.Errorf("ETag = %q", got)
	}
}

// ---------------------------------------------------------------- crear

func TestCrearDevuelve201ConLocationYETag(t *testing.T) {
	rec := llamar(t, &repoFalso{}, editor(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo("[" + heroValido + "]")})
	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/api/v1/pages/"+idDePagina.String() {
		t.Errorf("Location = %q", got)
	}
	if got := rec.Header().Get("ETag"); got != `"1"` {
		t.Errorf("ETag = %q", got)
	}
}

func TestUnSlugOcupadoEs409(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errSlugOcupado}, editor(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo("[]")})
	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestElCuerpoNoAceptaCamposQueNoSonDelCliente(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, editor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/pages",
		cuerpo: `{"slug":"a","title":"A","blocks":[],"version":9}`,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.escrito != nil {
		t.Error("se escribio con un campo desconocido")
	}
}

func TestLosCamposDeLaPaginaSeValidanTodosJuntos(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, editor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/pages",
		cuerpo: `{"slug":"Con Espacios","title":"","seoTitle":"` + strings.Repeat("x", 121) + `"}`,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	esperados := []string{"blocks:required", "seoTitle:max", "slug:format", "title:required"}
	if got := campos(problema(t, rec)); !slices.Equal(got, esperados) {
		t.Errorf("errores = %v, se esperaban %v", got, esperados)
	}
	if repo.escrito != nil {
		t.Error("se escribio una pagina invalida")
	}
}

// ---------------------------------------------------------------- bloques

// Criterio 4 de HU-009: props que no cumplen el esquema de su tipo son 400 con
// el indice del bloque y el campo, y no se escribe nada.
func TestPropsInvalidasSon400ConElIndiceDelBloqueYElCampoYNoSeEscribe(t *testing.T) {
	for _, metodo := range []string{http.MethodPost, http.MethodPut} {
		t.Run(metodo, func(t *testing.T) {
			repo := &repoFalso{}
			ruta := "/api/v1/pages"
			if metodo == http.MethodPut {
				ruta = rutaDePagina("")
			}

			bloques := `[` + heroValido + `,
				{"id":"b2","type":"hero","props":{"title":42}},
				{"id":"b3","type":"features","props":{"items":[{"title":"Uno"},{"text":"sin titulo"}]}}]`

			rec := llamar(t, repo, editor(), peticion{metodo: metodo, ruta: ruta, cuerpo: cuerpo(bloques), ifMatch: `"1"`})

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
			}
			esperados := []string{"blocks[1].props.title:type", "blocks[2].props.items[1].title:required"}
			if got := campos(problema(t, rec)); !slices.Equal(got, esperados) {
				t.Errorf("errores = %v, se esperaban %v", got, esperados)
			}
			if repo.escrito != nil {
				t.Error("se escribio con un bloque invalido")
			}
		})
	}
}

// Criterio 5 de HU-009.
func TestUnTipoQueNoEstaEnElCatalogoEs400(t *testing.T) {
	repo := &repoFalso{}
	rec := llamar(t, repo, editor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/pages",
		cuerpo: cuerpo(`[{"id":"b1","type":"carrusel","props":{}}]`),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	p := problema(t, rec)
	if got := campos(p); !slices.Equal(got, []string{"blocks[0].type:unknown"}) {
		t.Fatalf("errores = %v", got)
	}
	// El mensaje dice que tipos hay: sin eso, el error solo dice "no".
	if !strings.Contains(p.Errors[0].Message, "features, hero, texto") {
		t.Errorf("mensaje = %q", p.Errors[0].Message)
	}
	if repo.escrito != nil {
		t.Error("se escribio con un tipo desconocido")
	}
}

func TestLaFormaDeCadaBloqueSeExige(t *testing.T) {
	bloques := `[
		{"id":"","type":"hero","props":{"title":"a"}},
		{"id":"b2","type":"","props":{}},
		{"id":"b3","type":"texto","props":null},
		{"id":"b3","type":"texto","props":{"body":"a","color":"rojo"}},
		{"id":"b5","type":"hero","props":{"title":"a","cta":{"label":"x","href":"javascript:alert(1)"}}}]`

	rec := llamar(t, &repoFalso{}, editor(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo(bloques)})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	esperados := []string{
		"blocks[0].id:required",
		"blocks[1].type:required",
		"blocks[2].props:required",
		"blocks[3].id:duplicate",
		"blocks[3].props.color:unknown",
		"blocks[4].props.cta.href:format",
	}
	if got := campos(problema(t, rec)); !slices.Equal(got, esperados) {
		t.Errorf("errores = %v\nse esperaban %v", got, esperados)
	}
}

func TestUnaPaginaSinBloquesSeAceptaSiSeMandaVacia(t *testing.T) {
	rec := llamar(t, &repoFalso{}, editor(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo("[]")})
	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMasDeCienBloquesEs400(t *testing.T) {
	partes := make([]string, 101)
	for i := range partes {
		partes[i] = `{"id":"b` + strconv.Itoa(i) + `","type":"texto","props":{"body":"a"}}`
	}
	rec := llamar(t, &repoFalso{}, editor(), peticion{metodo: http.MethodPost, ruta: "/api/v1/pages", cuerpo: cuerpo("[" + strings.Join(partes, ",") + "]")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d", rec.Code)
	}
	if got := campos(problema(t, rec)); !slices.Equal(got, []string{"blocks:max"}) {
		t.Errorf("errores = %v", got)
	}
}

// ---------------------------------------------------------------- publicar

func TestPublicarPasaLaVersionPedidaYNoMandaETag(t *testing.T) {
	repo := &repoFalso{pagina_: Pagina{Id: idDePagina, Version: 4, Blocks: []Bloque{}}}
	rec := llamar(t, repo, con("content.page.publish"), peticion{
		metodo: http.MethodPost, ruta: rutaDePagina("/publish"),
		cuerpo: `{"versionId":"` + idDeVersion.String() + `"}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if repo.publicada != idDeVersion {
		t.Errorf("version publicada = %v", repo.publicada)
	}
	if got := rec.Header().Get("ETag"); got != "" {
		t.Errorf("publicar mando ETag %q: invita a pensar que cambio el borrador", got)
	}
}

func TestPublicarUnaVersionDeOtraPaginaEs400NombrandoElCampo(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errVersionAjena}, con("content.page.publish"), peticion{
		metodo: http.MethodPost, ruta: rutaDePagina("/publish"),
		cuerpo: `{"versionId":"` + idDeVersion.String() + `"}`,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if got := campos(problema(t, rec)); !slices.Equal(got, []string{"versionId:not_found"}) {
		t.Errorf("errores = %v", got)
	}
}

func TestBorrarEs204YUnaPaginaQueNoExisteEs404(t *testing.T) {
	rec := llamar(t, &repoFalso{}, editor(), peticion{metodo: http.MethodDelete, ruta: rutaDePagina("")})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}

	rec = llamar(t, &repoFalso{err: errNoExiste}, editor(), peticion{metodo: http.MethodDelete, ruta: rutaDePagina("")})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}
