package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// muxEspia no enruta nada: solo anota los patrones que le pasan. Sirve para
// preguntarle al codigo generado que rutas describe el contrato, sin leer el
// YAML ni depender de un parser.
type muxEspia struct{ patrones []string }

func (m *muxEspia) HandleFunc(patron string, _ func(http.ResponseWriter, *http.Request)) {
	m.patrones = append(m.patrones, patron)
}

func (m *muxEspia) ServeHTTP(http.ResponseWriter, *http.Request) {}

// El agujero que esta prueba tapa: la interfaz generada garantiza que los
// HANDLERS cuadren con el contrato, pero los PATRONES se escriben a mano en
// Routes(). Montar el login en `/auth/loguin` compila igual.
func TestLasRutasMontadasSonExactamenteLasDelContrato(t *testing.T) {
	espia := &muxEspia{}
	HandlerFromMuxWithBaseURL(&Module{}, espia, "/api/v1")

	r := httpx.NewRouter()
	(&Module{}).Routes(r)

	montadas := make([]string, 0)
	for _, rt := range r.Routes() {
		montadas = append(montadas, rt.Method+" "+rt.Pattern)
	}

	slices.Sort(montadas)
	slices.Sort(espia.patrones)

	if !slices.Equal(montadas, espia.patrones) {
		t.Errorf("las rutas montadas no son las del contrato\n  contrato: %v\n  montadas: %v",
			espia.patrones, montadas)
	}
}

// moduloDePrueba arma el modulo con su service falso. El emisor va de verdad:
// lo que interesa de estos handlers es justo que pongan las cookies.
func moduloDePrueba(t *testing.T) (*Module, *repoFalso) {
	t.Helper()
	svc, repo := servicio(t, true)
	return &Module{svc: svc, emisor: auth.NewEmisor(true)}, repo
}

// llamarModulo monta las rutas de verdad y mete al actor en el contexto, que es
// lo que hace en produccion el middleware de sesion.
func llamarModulo(t *testing.T, m *Module, actor *rbac.Actor, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	t.Helper()

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

	var body *strings.Reader
	if cuerpo != "" {
		body = strings.NewReader(cuerpo)
	} else {
		body = strings.NewReader("")
	}

	req := httptest.NewRequest(metodo, ruta, body)
	req.Header.Set("Content-Type", "application/json")
	// El contrato declara la cabecera CSRF como obligatoria, asi que el codigo
	// generado la exige al enlazar. Quien la COMPRUEBA es el middleware, que en
	// esta prueba no corre.
	req.Header.Set(auth.CabeceraCSRF, "el-token")

	rec := httptest.NewRecorder()
	observ.Chain(r.Handler(), observ.TraceID, inyectar).ServeHTTP(rec, req)
	return rec
}

func TestElLoginResponde204YLasCuatroCookies(t *testing.T) {
	m, _ := moduloDePrueba(t)
	if _, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "ana@casa.com", Password: passDePrueba, DisplayName: "Ana", Roles: []string{RolAdmin},
	}); err != nil {
		t.Fatalf("Crear: %v", err)
	}

	rec := llamarModulo(t, m, nil, http.MethodPost, "/api/v1/auth/login",
		`{"email":"ana@casa.com","password":"`+passDePrueba+`"}`)

	// 204 y sin cuerpo: el perfil se pide con `me`, para que haya UN solo lugar
	// que defina que sabe el frontend del usuario.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d, se esperaba 204: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("el login devolvio cuerpo: %s", rec.Body.String())
	}

	puestas := map[string]bool{}
	for _, c := range rec.Result().Cookies() {
		puestas[c.Name] = true
	}
	for _, nombre := range []string{auth.CookieAT, auth.CookieRT, auth.CookieCSRF, auth.CookieHasSession} {
		if !puestas[nombre] {
			t.Errorf("no se emitio la cookie %s", nombre)
		}
	}
}

// Un login fallido no puede dejar ninguna cookie: media sesion en el navegador
// es un cliente que cree tenerla y recibe 401 en todo.
func TestUnLoginFallidoNoDejaNingunaCookie(t *testing.T) {
	m, _ := moduloDePrueba(t)
	if _, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "ana@casa.com", Password: passDePrueba, DisplayName: "Ana", Roles: []string{RolAdmin},
	}); err != nil {
		t.Fatalf("Crear: %v", err)
	}

	rec := llamarModulo(t, m, nil, http.MethodPost, "/api/v1/auth/login",
		`{"email":"ana@casa.com","password":"esta-no-es-la-buena-tampoco"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d, se esperaba 401", rec.Code)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("un login fallido dejo %d cookies", len(cookies))
	}
}

// Un campo de mas es un 400, no algo que se ignora en silencio. Lo declara el
// contrato con additionalProperties: false.
func TestUnCuerpoConCamposDeMasEs400(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := llamarModulo(t, m, nil, http.MethodPost, "/api/v1/auth/login",
		`{"email":"ana@casa.com","password":"`+passDePrueba+`","roles":["superadmin"]}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
}

func TestMeDevuelveElPerfilDeQuienTieneLaSesion(t *testing.T) {
	m, _ := moduloDePrueba(t)
	u, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "ana@casa.com", Password: passDePrueba, DisplayName: "Ana", Roles: []string{RolAdmin},
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}

	rec := llamarModulo(t, m, &rbac.Actor{ID: u.ID}, http.MethodGet, "/api/v1/auth/me", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}

	var p Perfil
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	if p.Email != "ana@casa.com" || p.DisplayName != "Ana" {
		t.Errorf("perfil = %+v", p)
	}
	if p.Id.String() != u.ID {
		t.Errorf("id = %q, se esperaba %q", p.Id, u.ID)
	}
}

// Sin sesion, `me` es 401 y no un perfil vacio. Lo corta RequireSession.
func TestMeSinSesionEs401(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := llamarModulo(t, m, nil, http.MethodGet, "/api/v1/auth/me", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d, se esperaba 401", rec.Code)
	}
}

// Las listas vacias salen como `[]` y JAMAS como `null`. El contrato promete un
// array, y un cliente que hace `.includes()` sobre null revienta.
//
// Se mira el JSON crudo y no el tipo deserializado: `json.Unmarshal` deja un
// slice nil tanto con `[]` como con `null`, asi que una prueba que compare
// `len(p.Permissions) == 0` pasa con los dos. Lo descubrio una mutacion.
func TestLasListasVaciasDelPerfilSalenComoArrayYNoComoNull(t *testing.T) {
	m, repo := moduloDePrueba(t)
	u, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "sola@casa.com", Password: passDePrueba, DisplayName: "Sin roles",
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	// Una cuenta recien creada a la que todavia no se le concedio nada: es el
	// unico caso en el que las dos listas salen vacias a la vez.
	repo.permisos = &[]string{}

	rec := llamarModulo(t, m, &rbac.Actor{ID: u.ID}, http.MethodGet, "/api/v1/auth/me", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}

	cuerpo := rec.Body.String()
	if strings.Contains(cuerpo, `"roles":null`) || strings.Contains(cuerpo, `"permissions":null`) {
		t.Errorf("una lista vacia salio como null: %s", cuerpo)
	}
	if !strings.Contains(cuerpo, `"roles":[]`) || !strings.Contains(cuerpo, `"permissions":[]`) {
		t.Errorf("no salieron como array vacio: %s", cuerpo)
	}
}

// El manejador de errores por omision de lo generado responde con http.Error:
// texto plano y sin traceId, en medio de una API que promete problem+json.
func TestElLoginSinLaCabeceraCsrfSaleComoProblemJson(t *testing.T) {
	m, _ := moduloDePrueba(t)

	r := httpx.NewRouter()
	m.Routes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"ana@casa.com","password":"`+passDePrueba+`"}`))
	req.Header.Set("Content-Type", "application/json")
	// Sin la cabecera: en produccion la corta el middleware con un 403, pero si
	// llegara hasta el enlace del codigo generado tiene que salir con la forma
	// unica de error igual.
	rec := httptest.NewRecorder()
	observ.Chain(r.Handler(), observ.TraceID).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q; lo generado responde text/plain si no se le cambia el manejador", ct)
	}

	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	if p.TraceID == "" {
		t.Error("el problem no lleva traceId")
	}
}
