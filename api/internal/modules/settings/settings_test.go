package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

const idDeJuan = "3f1c9b2e-7d5a-4c81-9e0f-2a6b8c4d1e33"

// repoFalso reemplaza a Postgres. Las pruebas del molde afirman sobre lo que el
// service LE PIDE al repositorio —el limite, el orden, el filtro— y eso se ve
// mejor aqui que contra una base real.
type repoFalso struct {
	items []Setting
	total int64
	err   error

	// lo que quedo registrado de la ultima llamada
	params paging.Params
	sello  audit.Fields
}

func (r *repoFalso) listar(context.Context, bool) ([]Setting, error) {
	return r.items, r.err
}

func (r *repoFalso) pagina(_ context.Context, p paging.Params) ([]Setting, int64, error) {
	r.params = p
	return r.items, r.total, r.err
}

func (r *repoFalso) obtener(context.Context, string) (Setting, error) {
	if r.err != nil {
		return Setting{}, r.err
	}
	return r.items[0], nil
}

// crear pasa por platform/audit de verdad: es lo que permite comprobar que el
// actor de la peticion llega hasta el sello sin una base de datos de por medio.
func (r *repoFalso) crear(ctx context.Context, s Setting) (Setting, error) {
	sello, err := audit.ForInsert(ctx)
	if err != nil {
		return Setting{}, err
	}
	r.sello = sello

	if r.err != nil {
		return Setting{}, r.err
	}
	s.Version = 1
	s.UpdatedAt = time.Now()
	return s, nil
}

func (r *repoFalso) actualizar(ctx context.Context, s Setting) (Setting, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Setting{}, err
	}
	r.sello = sello

	if r.err != nil {
		return Setting{}, r.err
	}
	s.Version++
	return s, nil
}

func unSetting(key string) Setting {
	return Setting{Key: key, Value: json.RawMessage(`{"a":1}`), Version: 7, UpdatedAt: time.Now()}
}

// servidor monta el modulo con la misma cadena que en produccion. Sin el
// traceId las respuestas de error no serian las reales.
func servidor(repo repositorio, actor *rbac.Actor) http.Handler {
	m := &Module{svc: &Service{repo: repo}}
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

func lector() *rbac.Actor {
	return &rbac.Actor{ID: idDeJuan, Permissions: []string{"settings.read"}}
}

func escritor() *rbac.Actor {
	return &rbac.Actor{ID: idDeJuan, Permissions: []string{"settings.read", "settings.write"}}
}

type peticion struct {
	metodo  string
	ruta    string
	cuerpo  string
	ifMatch string
}

func llamar(t *testing.T, repo repositorio, actor *rbac.Actor, p peticion) *httptest.ResponseRecorder {
	t.Helper()

	var cuerpo *strings.Reader
	if p.cuerpo != "" {
		cuerpo = strings.NewReader(p.cuerpo)
	} else {
		cuerpo = strings.NewReader("")
	}

	req := httptest.NewRequest(p.metodo, p.ruta, cuerpo)
	if p.cuerpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.ifMatch != "" {
		req.Header.Set("If-Match", p.ifMatch)
	}

	rec := httptest.NewRecorder()
	servidor(repo, actor).ServeHTTP(rec, req)
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

// ---------------------------------------------------------------- coleccion

func TestLaColeccionSaleConElEnvoltorioYNuncaComoArray(t *testing.T) {
	repo := &repoFalso{items: []Setting{unSetting("site.brand")}, total: 1}

	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/settings"})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}

	if strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Fatalf("la coleccion salio como array pelado: %s", rec.Body.String())
	}

	var pagina paging.Page[Setting]
	if err := json.Unmarshal(rec.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("cuerpo = %q", rec.Body.String())
	}
	if len(pagina.Content) != 1 || pagina.Content[0].Key != "site.brand" {
		t.Errorf("content = %+v", pagina.Content)
	}
	if pagina.Page.Size != paging.DefaultSize || pagina.Page.TotalElements != 1 || pagina.Page.TotalPages != 1 {
		t.Errorf("page = %+v", pagina.Page)
	}
}

// El criterio del checklist, ejercitado por HTTP y no solo en el paquete.
func TestUnTamanoAbsurdoDevuelveElTopeYLaConsultaLoRespeta(t *testing.T) {
	repo := &repoFalso{total: 0}

	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/settings?size=1000000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}

	var pagina paging.Page[Setting]
	_ = json.Unmarshal(rec.Body.Bytes(), &pagina)

	if pagina.Page.Size != 100 {
		t.Errorf("page.size = %d, se esperaba 100", pagina.Page.Size)
	}
	if repo.params.Limit() != 100 {
		t.Errorf("el repositorio recibio limit = %d; el tope tiene que llegar a la consulta", repo.params.Limit())
	}
}

// El otro criterio: un sort fuera de la lista blanca no llega a ser SQL.
func TestUnSortInvalidoEs400YNo500(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(),
		peticion{metodo: http.MethodGet, ruta: "/api/v1/settings?sort=passwordHash,desc"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); p.Code != httpx.CodeValidationFailed {
		t.Errorf("code = %q", p.Code)
	}
}

func TestUnFiltroNoDeclaradoEs400(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(),
		peticion{metodo: http.MethodGet, ruta: "/api/v1/settings?createdBy=otro"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
}

func TestElOrdenYElFiltroLleganResueltosAColumnaReal(t *testing.T) {
	repo := &repoFalso{total: 1, items: []Setting{unSetting("a")}}

	llamar(t, repo, lector(), peticion{
		metodo: http.MethodGet,
		ruta:   "/api/v1/settings?sort=updatedAt,desc&isPublic=true",
	})

	if got := repo.params.OrderBy(); got != "order by updated_at desc, key asc" {
		t.Errorf("orderBy = %q", got)
	}
	clausula, args := repo.params.Where(1)
	if clausula != "where is_public = $1" || len(args) != 1 || args[0] != true {
		t.Errorf("where = %q, args = %#v", clausula, args)
	}
}

// La landing pide la configuracion entera en cada carga: ahi el envoltorio
// estorbaria en vez de ayudar, y por eso esta escrito que es una excepcion.
func TestLaConfiguracionPublicaSigueSaliendoComoMapaPlano(t *testing.T) {
	repo := &repoFalso{items: []Setting{unSetting("site.brand")}}

	rec := llamar(t, repo, nil, peticion{metodo: http.MethodGet, ruta: "/api/v1/public/settings"})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d", rec.Code)
	}

	var mapa map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &mapa); err != nil {
		t.Fatalf("cuerpo = %q", rec.Body.String())
	}
	if _, ok := mapa["site.brand"]; !ok {
		t.Errorf("mapa = %v", mapa)
	}
}

// ------------------------------------------------------------- autorizacion

func TestSinSesionEs401(t *testing.T) {
	for _, p := range []peticion{
		{metodo: http.MethodGet, ruta: "/api/v1/settings"},
		{metodo: http.MethodGet, ruta: "/api/v1/settings/site.brand"},
		{metodo: http.MethodPost, ruta: "/api/v1/settings", cuerpo: `{"key":"a","value":1}`},
		{metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand", cuerpo: `{"value":1}`, ifMatch: `"7"`},
	} {
		rec := llamar(t, &repoFalso{}, nil, p)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: estado = %d, se esperaba 401", p.metodo, p.ruta, rec.Code)
		}
	}
}

func TestConSesionSinPermisoDeEscrituraEs403(t *testing.T) {
	rec := llamar(t, &repoFalso{}, lector(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings", cuerpo: `{"key":"a.b","value":1}`,
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d, se esperaba 403: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); p.Code != httpx.CodeForbidden {
		t.Errorf("code = %q", p.Code)
	}
}

// ------------------------------------------------------------- concurrencia

func TestLeerDevuelveElETagQueDespuesSePideEnIfMatch(t *testing.T) {
	repo := &repoFalso{items: []Setting{unSetting("site.brand")}}

	rec := llamar(t, repo, lector(), peticion{metodo: http.MethodGet, ruta: "/api/v1/settings/site.brand"})
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d", rec.Code)
	}
	if got := rec.Header().Get("ETag"); got != `"7"` {
		t.Errorf("ETag = %q, se esperaba \"7\"", got)
	}
}

// Sin If-Match, cada guardado descuidado seria una sobrescritura silenciosa.
func TestReemplazarSinIfMatchEs400(t *testing.T) {
	rec := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand", cuerpo: `{"value":1}`,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	p := problema(t, rec)
	if len(p.Errors) != 1 || p.Errors[0].Field != "If-Match" {
		t.Errorf("errors = %+v; tiene que nombrar la cabecera que falta", p.Errors)
	}
}

// `*` significa "cualquiera": es la sobrescritura incondicional que esto existe
// para impedir.
func TestReemplazarConIfMatchComodinEs400(t *testing.T) {
	rec := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand",
		cuerpo: `{"value":1}`, ifMatch: "*",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
}

func TestUnIfMatchViejoEs409(t *testing.T) {
	repo := &repoFalso{err: errVersion}

	rec := llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand",
		cuerpo: `{"value":{"a":2}}`, ifMatch: `"3"`,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); p.Code != httpx.CodeConflict {
		t.Errorf("code = %q", p.Code)
	}
}

func TestReemplazarUnaClaveQueNoExisteEs404(t *testing.T) {
	repo := &repoFalso{err: errNoExiste}

	rec := llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/no.existe",
		cuerpo: `{"value":1}`, ifMatch: `"1"`,
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404: %s", rec.Code, rec.Body.String())
	}
}

func TestLaVersionQueLlegaAlRepositorioEsLaDelIfMatch(t *testing.T) {
	repo := &repoFalso{}

	rec := llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand",
		cuerpo: `{"value":{"a":2},"isPublic":true}`, ifMatch: `"7"`,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	// El repositorio recibio la 7 y devolvio la 8: el ETag de la respuesta ya
	// es el de la version nueva.
	if got := rec.Header().Get("ETag"); got != `"8"` {
		t.Errorf("ETag = %q, se esperaba \"8\"", got)
	}
}

// -------------------------------------------------------------------- alta

func TestCrearDevuelve201ConLocationYETag(t *testing.T) {
	rec := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings",
		cuerpo: `{"key":"site.footer","value":{"texto":"hola"},"isPublic":true}`,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d, se esperaba 201: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/api/v1/settings/site.footer" {
		t.Errorf("Location = %q", got)
	}
	if got := rec.Header().Get("ETag"); got != `"1"` {
		t.Errorf("ETag = %q", got)
	}
}

func TestUnaClaveDuplicadaEs409(t *testing.T) {
	rec := llamar(t, &repoFalso{err: errYaExiste}, escritor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings",
		cuerpo: `{"key":"site.brand","value":1}`,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409: %s", rec.Code, rec.Body.String())
	}
}

func TestUnaClaveMalFormadaEs400YNombraElCampo(t *testing.T) {
	for _, clave := range []string{"", "Site.Brand", "site brand", "site..brand", ".site"} {
		cuerpo, _ := json.Marshal(entrada{Key: clave, Value: json.RawMessage(`1`)})

		rec := llamar(t, &repoFalso{}, escritor(), peticion{
			metodo: http.MethodPost, ruta: "/api/v1/settings", cuerpo: string(cuerpo),
		})

		if rec.Code != http.StatusBadRequest {
			t.Errorf("clave %q: estado = %d, se esperaba 400", clave, rec.Code)
			continue
		}
		if p := problema(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "key" {
			t.Errorf("clave %q: errors = %+v", clave, p.Errors)
		}
	}
}

// El cuerpo no se decodifica sobre la entidad: `version` y `updatedBy` no son
// del cliente, y mandarlos tiene que ser un 400 y no un campo ignorado.
func TestElCuerpoNoAceptaCamposQueNoSonDelCliente(t *testing.T) {
	rec := llamar(t, &repoFalso{}, escritor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings",
		cuerpo: `{"key":"a.b","value":1,"version":99}`,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "version" {
		t.Errorf("errors = %+v", p.Errors)
	}
}

// --------------------------------------------------------------- auditoria

// La auditoria se llena sola: el handler y el service no la mencionan, y aun
// asi el actor de la peticion llega al sello.
func TestElActorDeLaPeticionLlegaALaAuditoriaSinQueNadieLoAsigne(t *testing.T) {
	repo := &repoFalso{}

	llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPost, ruta: "/api/v1/settings",
		cuerpo: `{"key":"site.footer","value":1}`,
	})

	var esperado pgtype.UUID
	if err := esperado.Scan(idDeJuan); err != nil {
		t.Fatal(err)
	}

	porColumna := map[string]any{}
	for i, c := range repo.sello.Columns {
		porColumna[c] = repo.sello.Values[i]
	}

	for _, c := range []string{"created_at", "created_by", "updated_at", "updated_by"} {
		if _, ok := porColumna[c]; !ok {
			t.Fatalf("el alta no sello %q: columnas = %v", c, repo.sello.Columns)
		}
	}
	if porColumna["created_by"] != esperado || porColumna["updated_by"] != esperado {
		t.Errorf("el sello no lleva al actor de la peticion: %#v", porColumna)
	}
}

// Y una modificacion no toca quien creo la fila.
func TestLaModificacionNoSellaLasColumnasDeCreacion(t *testing.T) {
	repo := &repoFalso{}

	llamar(t, repo, escritor(), peticion{
		metodo: http.MethodPut, ruta: "/api/v1/settings/site.brand",
		cuerpo: `{"value":1}`, ifMatch: `"7"`,
	})

	for _, c := range repo.sello.Columns {
		if c == "created_at" || c == "created_by" {
			t.Errorf("la modificacion sello %q: eso borra quien creo la fila", c)
		}
	}
	if len(repo.sello.Columns) != 2 {
		t.Errorf("columnas = %v, se esperaban solo las de modificacion", repo.sello.Columns)
	}
}
