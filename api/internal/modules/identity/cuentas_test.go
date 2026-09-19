package identity

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Las pruebas del molde sobre el dashboard de cuentas (HU-018), sin base. Lo
// que solo existe en SQL —la transaccion de deshabilitar, el `citext`, la
// auditoria— lo prueba repo_integracion_test.go.

// --- el doble ---------------------------------------------------------------

func (r *repoFalso) cuentas(_ context.Context, p paging.Params) ([]UsuarioConRoles, int64, error) {
	todas := make([]UsuarioConRoles, 0, len(r.porEmail))
	for _, f := range r.porEmail {
		todas = append(todas, UsuarioConRoles{Usuario: f.usuario, Roles: slices.Clone(r.roles[f.usuario.ID])})
	}
	slices.SortFunc(todas, func(a, b UsuarioConRoles) int { return strings.Compare(a.Email, b.Email) })

	total := int64(len(todas))
	desde := min(p.Offset(), len(todas))
	hasta := min(desde+p.Limit(), len(todas))
	return todas[desde:hasta], total, nil
}

func (r *repoFalso) editar(_ context.Context, id string, version int, m ModificacionDeUsuario) (Usuario, error) {
	f := r.filaPorID(id)
	if f == nil {
		return Usuario{}, errNoExiste
	}
	if f.usuario.Version != version {
		return Usuario{}, errVersion
	}
	if otra, ya := r.porEmail[strings.ToLower(m.Email)]; ya && otra != f {
		return Usuario{}, errYaExiste
	}

	delete(r.porEmail, strings.ToLower(f.usuario.Email))
	f.usuario.Email = m.Email
	f.usuario.DisplayName = m.DisplayName
	f.usuario.Version++
	r.porEmail[strings.ToLower(m.Email)] = f
	return f.usuario, nil
}

func (r *repoFalso) habilitar(_ context.Context, id string) (Usuario, error) {
	f := r.filaPorID(id)
	if f == nil {
		return Usuario{}, errNoExiste
	}
	if !f.usuario.Enabled {
		f.usuario.Enabled = true
		f.usuario.Version++
	}
	return f.usuario, nil
}

func (r *repoFalso) catalogoDeRoles(_ context.Context, p paging.Params) ([]RolConPermisos, int64, error) {
	todos := []RolConPermisos{
		{Key: RolAdmin, Name: "Administracion", Permisos: []rbac.Permission{{Key: "content.page.read", Desc: "Ver"}}},
		{Key: RolSuperadmin, Name: "Superadministracion", Permisos: []rbac.Permission{
			{Key: "content.page.read", Desc: "Ver"},
			{Key: "identity.role.assign", Desc: "Repartir", Sensitive: true},
		}},
		{Key: RolViewer, Name: "Solo lectura"},
	}
	return todos, int64(len(todos)), nil
}

// --- las peticiones ---------------------------------------------------------

// todosLosPermisos es el superadmin: los cuatro de identity.
var todosLosPermisos = []string{
	"identity.user.read", "identity.user.write", "identity.role.read", "identity.role.assign",
}

func con(permisos ...string) *rbac.Actor {
	return &rbac.Actor{ID: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", Permissions: permisos}
}

// pedir es llamarModulo con cabeceras: el guardado necesita `If-Match`.
func pedir(t *testing.T, m *Module, actor *rbac.Actor, metodo, ruta, cuerpo string, cabeceras ...string) *httptest.ResponseRecorder {
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

	req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CabeceraCSRF, "el-token")
	for i := 0; i+1 < len(cabeceras); i += 2 {
		req.Header.Set(cabeceras[i], cabeceras[i+1])
	}

	rec := httptest.NewRecorder()
	observ.Chain(r.Handler(), observ.TraceID, inyectar).ServeHTTP(rec, req)
	return rec
}

func cuentaDe(t *testing.T, rec *httptest.ResponseRecorder) Cuenta {
	t.Helper()
	var c Cuenta
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	return c
}

func problemaHTTP(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	return p
}

// anaEnElModulo da de alta a Ana por el service, sin roles.
func anaEnElModulo(t *testing.T) (*Module, *repoFalso, Usuario) {
	t.Helper()
	m, repo := moduloDePrueba(t)
	u, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	return m, repo, u
}

// --- autorizacion -------------------------------------------------------------

// Las ocho rutas con el permiso que piden. Sin sesion, 401; con sesion y todos
// los permisos MENOS ese, 403. Lo segundo es lo que detecta una ruta montada
// con el permiso equivocado: con un actor sin ningun permiso, pedir el de al
// lado tambien daria 403.
var rutasDeCuentas = []struct {
	metodo, ruta, cuerpo, permiso string
}{
	{http.MethodGet, "/api/v1/users", "", "identity.user.read"},
	{http.MethodPost, "/api/v1/users", `{"email":"b@casa.com","displayName":"B","password":"` + claveDePrueba + `"}`, "identity.user.write"},
	{http.MethodGet, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", "", "identity.user.read"},
	{http.MethodPut, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", `{"email":"b@casa.com","displayName":"B"}`, "identity.user.write"},
	{http.MethodPost, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/disable", "", "identity.user.write"},
	{http.MethodPost, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/enable", "", "identity.user.write"},
	{http.MethodPut, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/roles/staff", "", "identity.role.assign"},
	{http.MethodDelete, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/roles/staff", "", "identity.role.assign"},
	{http.MethodGet, "/api/v1/roles", "", "identity.role.read"},
}

func TestCadaRutaDeCuentasSinSesionEs401(t *testing.T) {
	for _, rt := range rutasDeCuentas {
		t.Run(rt.metodo+" "+rt.ruta, func(t *testing.T) {
			m, _ := moduloDePrueba(t)
			rec := pedir(t, m, nil, rt.metodo, rt.ruta, rt.cuerpo, "If-Match", `"1"`)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("estado = %d, se esperaba 401: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCadaRutaDeCuentasSinSuPermisoEs403(t *testing.T) {
	for _, rt := range rutasDeCuentas {
		t.Run(rt.metodo+" "+rt.ruta, func(t *testing.T) {
			m, _ := moduloDePrueba(t)
			otros := slices.DeleteFunc(slices.Clone(todosLosPermisos), func(p string) bool { return p == rt.permiso })

			rec := pedir(t, m, con(otros...), rt.metodo, rt.ruta, rt.cuerpo, "If-Match", `"1"`)
			if rec.Code != http.StatusForbidden {
				t.Errorf("estado = %d, se esperaba 403 sin %s: %s", rec.Code, rt.permiso, rec.Body.String())
			}
		})
	}
}

// El criterio que separa los dos permisos: quien da de alta cuentas no puede
// por eso repartir poder, ni a otros ni a si mismo.
func TestCrearCuentasNoPermiteDarseRoles(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	actor := &rbac.Actor{ID: ana.ID, Permissions: []string{"identity.user.read", "identity.user.write"}}

	rec := pedir(t, m, actor, http.MethodPut, "/api/v1/users/"+ana.ID+"/roles/superadmin", "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d, se esperaba 403", rec.Code)
	}
	if roles := repo.roles[ana.ID]; len(roles) != 0 {
		t.Errorf("se asigno %v aun con el 403", roles)
	}
}

// --- listar ---------------------------------------------------------------------

func TestListarCuentasRespetaElTopeDeSize(t *testing.T) {
	m, _, _ := anaEnElModulo(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, "/api/v1/users?size=1000000", "")

	// El contrato declara `maximum: 100`, pero el servidor no valida contra el
	// YAML: quien lo recorta es paging. Si no, 1000000 llegaria a la consulta.
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	var pagina CuentasPage
	if err := json.Unmarshal(rec.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	if pagina.Page.Size != 100 {
		t.Errorf("page.size = %d, se esperaba el tope 100", pagina.Page.Size)
	}
	if len(pagina.Content) != 1 || pagina.Content[0].Email != emailDePrueba {
		t.Errorf("content = %+v", pagina.Content)
	}
}

func TestListarCuentasConUnOrdenFueraDeLaListaEs400(t *testing.T) {
	m, _ := moduloDePrueba(t)

	// `password_hash` es justo la columna que no tiene que poder ordenarse: el
	// orden de un hash filtra un bit de el por fila.
	for _, sort := range []string{"password_hash", "roles", "email,sideways"} {
		rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, "/api/v1/users?sort="+sort, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("sort=%s: estado = %d, se esperaba 400", sort, rec.Code)
		}
	}
}

// Ni el hash ni nada que se le parezca sale por el listado. Se mira el JSON
// crudo: un campo de mas en el tipo generado no lo detectaria el Unmarshal.
func TestNingunaRespuestaDeCuentasLlevaElHash(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	hash := repo.porEmail[emailDePrueba].hash

	for _, ruta := range []string{"/api/v1/users", "/api/v1/users/" + ana.ID} {
		rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, ruta, "")
		cuerpo := rec.Body.String()
		if strings.Contains(cuerpo, hash) || strings.Contains(cuerpo, "argon2") || strings.Contains(cuerpo, "password") {
			t.Errorf("%s deja ver la contrasena: %s", ruta, cuerpo)
		}
	}
}

// --- crear ----------------------------------------------------------------------

func TestCrearCuentaResponde201ConLocationYETag(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users",
		`{"email":"bea@casa.com","displayName":"Bea","password":"`+claveDePrueba+`"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	c := cuentaDe(t, rec)
	if loc := rec.Header().Get("Location"); loc != "/api/v1/users/"+c.Id.String() {
		t.Errorf("Location = %q", loc)
	}
	if et := rec.Header().Get("ETag"); et != `"1"` {
		t.Errorf("ETag = %q, se esperaba \"1\"", et)
	}
	if !c.Enabled {
		t.Error("la cuenta nacio deshabilitada")
	}
	if !strings.Contains(rec.Body.String(), `"roles":[]`) {
		t.Errorf("la cuenta tiene que nacer sin roles, y como [] y no null: %s", rec.Body.String())
	}
}

// El alta no recibe roles: repartirlos es otro permiso.
func TestCrearCuentaConRolesEs400(t *testing.T) {
	m, repo := moduloDePrueba(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users",
		`{"email":"bea@casa.com","displayName":"Bea","password":"`+claveDePrueba+`","roles":["superadmin"]}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
	}
	if len(repo.porEmail) != 0 {
		t.Error("se creo la cuenta aun con el 400")
	}
}

func TestCrearCuentaConContrasenaCortaNombraElCampo(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users",
		`{"email":"bea@casa.com","displayName":"Bea","password":"corta"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400", rec.Code)
	}
	p := problemaHTTP(t, rec)
	if len(p.Errors) != 1 || p.Errors[0].Field != "password" {
		t.Errorf("errors = %+v; tiene que nombrar password", p.Errors)
	}
}

func TestCrearCuentaConUnCorreoRepetidoEs409(t *testing.T) {
	m, _, _ := anaEnElModulo(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users",
		`{"email":"ANA@casa.com","displayName":"Otra Ana","password":"`+claveDePrueba+`"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409: %s", rec.Code, rec.Body.String())
	}
}

// --- leer y guardar ---------------------------------------------------------

func TestLeerUnaCuentaQueNoExisteEs404(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, "/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404", rec.Code)
	}
}

func TestLeerUnaCuentaDevuelveSusRolesYSuETag(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.roles[ana.ID] = []string{RolStaff}

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, "/api/v1/users/"+ana.ID, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if c := cuentaDe(t, rec); !slices.Equal(c.Roles, []string{RolStaff}) {
		t.Errorf("roles = %v", c.Roles)
	}
	if et := rec.Header().Get("ETag"); et != `"1"` {
		t.Errorf("ETag = %q", et)
	}
}

// Una cuenta sin roles los trae como `[]` y no como `null` en toda respuesta,
// no solo en el alta. Se mira el JSON crudo por la misma razon que en `me`.
func TestUnaCuentaSinRolesLosTraeComoArrayVacio(t *testing.T) {
	m, _, ana := anaEnElModulo(t)

	for _, ruta := range []string{"/api/v1/users", "/api/v1/users/" + ana.ID} {
		rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, ruta, "")
		if !strings.Contains(rec.Body.String(), `"roles":[]`) {
			t.Errorf("%s: %s", ruta, rec.Body.String())
		}
	}
}

func TestGuardarCuentaSubeLaVersion(t *testing.T) {
	m, _, ana := anaEnElModulo(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID,
		`{"email":"ana.maria@casa.com","displayName":"  Ana Maria  "}`, "If-Match", `"1"`)

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	c := cuentaDe(t, rec)
	if c.Email != "ana.maria@casa.com" || c.DisplayName != "Ana Maria" {
		t.Errorf("cuenta = %+v; el nombre se guarda recortado", c)
	}
	if et := rec.Header().Get("ETag"); et != `"2"` {
		t.Errorf("ETag = %q, se esperaba \"2\"", et)
	}
}

func TestGuardarCuentaConUnIfMatchViejoEs409(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.porEmail[emailDePrueba].usuario.Version = 3

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID,
		`{"email":"ana@casa.com","displayName":"Otra"}`, "If-Match", `"2"`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409: %s", rec.Code, rec.Body.String())
	}
	if repo.porEmail[emailDePrueba].usuario.DisplayName != nombreDePrueba {
		t.Error("el guardado con version vieja escribio igual")
	}
}

// El PUT es de dos campos. Cualquier otro es un 400: un PUT que aceptara
// `enabled` o `roles` haria de identity.user.write un permiso para repartir.
func TestGuardarCuentaNoAceptaNadaMasQueCorreoYNombre(t *testing.T) {
	for _, extra := range []string{
		`"password":"` + claveDePrueba + `"`, `"enabled":false`, `"roles":["superadmin"]`, `"id":"x"`, `"version":9`,
	} {
		t.Run(extra, func(t *testing.T) {
			m, _, ana := anaEnElModulo(t)
			rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID,
				`{"email":"ana@casa.com","displayName":"Ana",`+extra+`}`, "If-Match", `"1"`)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("estado = %d, se esperaba 400", rec.Code)
			}
		})
	}
}

func TestGuardarCuentaValidaElCorreo(t *testing.T) {
	m, _, ana := anaEnElModulo(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID,
		`{"email":"sin-arroba","displayName":"Ana"}`, "If-Match", `"1"`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400", rec.Code)
	}
	if p := problemaHTTP(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "email" {
		t.Errorf("errors = %+v", p.Errors)
	}
}

// --- habilitar y deshabilitar -------------------------------------------------

func TestDeshabilitarRevocaSusSesiones(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	if _, err := m.svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: emailDePrueba, Password: claveDePrueba, IP: "10.0.0.1",
	}); err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users/"+ana.ID+"/disable", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if c := cuentaDe(t, rec); c.Enabled {
		t.Error("sigue habilitada")
	}
	if vivas := repo.vivas(ana.ID); vivas != 0 {
		t.Errorf("quedaron %d sesiones vivas", vivas)
	}
}

func TestDeshabilitarAlUltimoSuperadminEs409(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.errAlDeshabilitar = errUltimoSuperadmin

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users/"+ana.ID+"/disable", "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409", rec.Code)
	}
	// El mensaje es lo que el dashboard muestra; tiene que decir que hacer.
	if p := problemaHTTP(t, rec); !strings.Contains(p.Detail, "superadmin") {
		t.Errorf("detail = %q", p.Detail)
	}
}

// Habilitar o deshabilitar dos veces es el mismo estado, no un error, y no le
// sube la version: nada cambio.
func TestHabilitarYDeshabilitarSonIdempotentes(t *testing.T) {
	m, _, ana := anaEnElModulo(t)
	admin := con(todosLosPermisos...)

	for _, paso := range []struct {
		accion     string
		habilitada bool
		version    string
	}{
		{"enable", true, `"1"`},
		{"disable", false, `"2"`},
		{"disable", false, `"2"`},
		{"enable", true, `"3"`},
		{"enable", true, `"3"`},
	} {
		rec := pedir(t, m, admin, http.MethodPost, "/api/v1/users/"+ana.ID+"/"+paso.accion, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: estado = %d: %s", paso.accion, rec.Code, rec.Body.String())
		}
		if c := cuentaDe(t, rec); c.Enabled != paso.habilitada {
			t.Errorf("%s: enabled = %v", paso.accion, c.Enabled)
		}
		if et := rec.Header().Get("ETag"); et != paso.version {
			t.Errorf("%s: ETag = %s, se esperaba %s", paso.accion, et, paso.version)
		}
	}
}

// --- roles ------------------------------------------------------------------------

func TestDarUnRolEsIdempotente(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)

	for range 2 {
		rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID+"/roles/staff", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("estado = %d, se esperaba 204: %s", rec.Code, rec.Body.String())
		}
	}
	if !slices.Equal(repo.roles[ana.ID], []string{RolStaff}) {
		t.Errorf("roles = %v", repo.roles[ana.ID])
	}
}

// En la URL, un rol que no existe es un recurso que no existe: 404, y no el
// 409 con el que contesta el alta de la siembra.
func TestDarUnRolQueNoExisteEs404(t *testing.T) {
	m, _, ana := anaEnElModulo(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPut, "/api/v1/users/"+ana.ID+"/roles/inventado", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404: %s", rec.Code, rec.Body.String())
	}
}

func TestQuitarleSuperadminAlUltimoEs409(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.errAlQuitarRol = errUltimoSuperadmin

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodDelete, "/api/v1/users/"+ana.ID+"/roles/superadmin", "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409", rec.Code)
	}
}

func TestListarRolesMarcaLosSensiblesYNuncaDaNull(t *testing.T) {
	m, _ := moduloDePrueba(t)

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodGet, "/api/v1/roles", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"permissions":null`) {
		t.Errorf("un rol sin permisos salio como null: %s", rec.Body.String())
	}

	var pagina RolesPage
	if err := json.Unmarshal(rec.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("cuerpo no JSON: %s", rec.Body.String())
	}
	var super *Rol
	for i := range pagina.Content {
		if pagina.Content[i].Key == RolSuperadmin {
			super = &pagina.Content[i]
		}
	}
	if super == nil {
		t.Fatalf("no vino el superadmin: %+v", pagina.Content)
	}
	for _, p := range super.Permissions {
		if (p.Key == "identity.role.assign") != p.Sensitive {
			t.Errorf("%s: sensitive = %v", p.Key, p.Sensitive)
		}
	}
}

// --- la sesion de una cuenta que ya no puede entrar ----------------------------

// Sin esto, deshabilitar a alguien le dejaba sus permisos hasta que caducara
// su `at`: quince minutos.
func TestUnaCuentaDeshabilitadaNoResuelveActor(t *testing.T) {
	s, _ := servicio(t, true)
	u := conUsuario(t, s, emailDePrueba)

	if _, err := s.Actor(context.Background(), u.ID); err != nil {
		t.Fatalf("antes de deshabilitar: %v", err)
	}
	if _, err := s.Deshabilitar(context.Background(), u.ID); err != nil {
		t.Fatalf("Deshabilitar: %v", err)
	}

	actor, err := s.Actor(context.Background(), u.ID)
	if err == nil {
		t.Fatalf("la cuenta deshabilitada sigue resolviendo permisos: %v", actor.Permissions)
	}
}

func TestLaCuentaSembradaNoResuelveActorFueraDeDesarrollo(t *testing.T) {
	s, repo := servicio(t, false)
	u := conUsuario(t, s, emailDePrueba)
	repo.porEmail[emailDePrueba].usuario.DevSeed = true

	if _, err := s.Actor(context.Background(), u.ID); err == nil {
		t.Fatal("la cuenta de desarrollo resuelve permisos fuera de desarrollo")
	}
}

// El 401 de punta a punta: una peticion con una sesion vigente de una cuenta
// deshabilitada llega como anonima, y el guard responde lo mismo que sin sesion.
func TestLaSesionVigenteDeUnaCuentaDeshabilitadaDa401(t *testing.T) {
	m, _, ana := anaEnElModulo(t)
	f, err := auth.NewFirmante("llave-de-prueba-de-identity")
	if err != nil {
		t.Fatalf("NewFirmante: %v", err)
	}
	at, err := f.Firmar(ana.ID, nil)
	if err != nil {
		t.Fatalf("Firmar: %v", err)
	}
	if _, err := m.svc.Deshabilitar(context.Background(), ana.ID); err != nil {
		t.Fatalf("Deshabilitar: %v", err)
	}

	r := httpx.NewRouter()
	m.Routes(r)
	sesion := auth.Sesion(f, m.Actor, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAT, Value: at})
	rec := httptest.NewRecorder()
	observ.Chain(r.Handler(), observ.TraceID, sesion).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d, se esperaba 401: %s", rec.Code, rec.Body.String())
	}
}
