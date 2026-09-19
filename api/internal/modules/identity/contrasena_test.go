package identity

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Las pruebas sin base de la contrasena temporal (HU-019). La regla de poder,
// la solicitud unica por cuenta y la transaccion viven en SQL y las prueba
// contrasena_integracion_test.go.

// --- el doble ---------------------------------------------------------------

func (r *repoFalso) pedirContrasena(_ context.Context, id, email, _, _ string) error {
	f, ok := r.porEmail[strings.ToLower(email)]
	if !ok || !f.usuario.Enabled {
		return nil
	}
	for _, s := range r.pendientes {
		if s.UserID == f.usuario.ID {
			return nil
		}
	}
	r.pendientes = append(r.pendientes, SolicitudPendiente{ID: id, UserID: f.usuario.ID, Email: f.usuario.Email, DisplayName: f.usuario.DisplayName})
	return nil
}

func (r *repoFalso) asignarContrasena(_ context.Context, _, userID, hash string) error {
	if r.errAlAsignar != nil {
		return r.errAlAsignar
	}
	f := r.filaPorID(userID)
	if f == nil {
		return errNoExiste
	}
	f.hash = hash
	f.usuario.MustChangePassword = true
	f.usuario.Version++
	_ = r.revocarSesionesDe(context.Background(), userID)
	r.pendientes = slices.DeleteFunc(r.pendientes, func(s SolicitudPendiente) bool { return s.UserID == userID })
	return nil
}

func (r *repoFalso) cambiarContrasena(_ context.Context, userID, hash string) error {
	f := r.filaPorID(userID)
	if f == nil {
		return errNoExiste
	}
	f.hash = hash
	f.usuario.MustChangePassword = false
	f.usuario.Version++
	return r.revocarSesionesDe(context.Background(), userID)
}

func (r *repoFalso) solicitudes(context.Context, paging.Params) ([]SolicitudPendiente, int64, error) {
	return slices.Clone(r.pendientes), int64(len(r.pendientes)), nil
}

// --- pedir ------------------------------------------------------------------------

// El criterio que mas importa del pedido: no delatar que correos existen. Se
// compara el cuerpo ENTERO, no solo el estado: un mensaje distinto delata igual.
func TestPedirContrasenaRespondeIgualExistaONoLaCuenta(t *testing.T) {
	m, repo, _ := anaEnElModulo(t)
	if _, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "apagada@casa.com", Password: claveDePrueba, DisplayName: "Apagada",
	}); err != nil {
		t.Fatalf("Crear: %v", err)
	}
	repo.porEmail["apagada@casa.com"].usuario.Enabled = false

	var cuerpos []string
	for _, email := range []string{emailDePrueba, "nadie@casa.com", "apagada@casa.com"} {
		rec := pedir(t, m, nil, http.MethodPost, "/api/v1/auth/password-reset", `{"email":"`+email+`"}`)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("%s: estado = %d, se esperaba 202", email, rec.Code)
		}
		cuerpos = append(cuerpos, rec.Body.String())
	}
	if cuerpos[0] != cuerpos[1] || cuerpos[1] != cuerpos[2] {
		t.Errorf("los cuerpos difieren, y eso delata la cuenta:\n%s", strings.Join(cuerpos, "\n"))
	}
	if len(repo.pendientes) != 1 || repo.pendientes[0].Email != emailDePrueba {
		t.Errorf("pendientes = %+v; solo la cuenta habilitada deja solicitud", repo.pendientes)
	}
}

func TestPedirContrasenaConUnCorreoMalFormadoEs400(t *testing.T) {
	m, _ := moduloDePrueba(t)
	rec := pedir(t, m, nil, http.MethodPost, "/api/v1/auth/password-reset", `{"email":"sin-arroba"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d, se esperaba 400", rec.Code)
	}
}

// --- asignar ------------------------------------------------------------------------

func TestAsignarContrasenaLaDejaTemporalYRevocaSusSesiones(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	if _, err := m.svc.IniciarSesion(context.Background(), IntentoDeSesion{Email: emailDePrueba, Password: claveDePrueba}); err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}
	_ = m.svc.PedirContrasena(context.Background(), emailDePrueba, "", "")
	hashViejo := repo.porEmail[emailDePrueba].hash

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users/"+ana.ID+"/password",
		`{"password":"una temporal bien larga"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	f := repo.porEmail[emailDePrueba]
	if f.hash == hashViejo || strings.Contains(f.hash, "una temporal") {
		t.Error("no se guardo un hash nuevo")
	}
	if !f.usuario.MustChangePassword {
		t.Error("la contrasena asignada no quedo como temporal")
	}
	if n := repo.vivas(ana.ID); n != 0 {
		t.Errorf("quedaron %d sesiones vivas", n)
	}
	if len(repo.pendientes) != 0 {
		t.Error("la solicitud sigue pendiente")
	}
}

// La regla de poder vive en el SQL; aqui se comprueba que su sentinela sale
// como 403 con un mensaje que dice que hacer.
func TestAsignarAQuienTieneMasPoderEs403(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.errAlAsignar = errMasPoder

	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users/"+ana.ID+"/password",
		`{"password":"una temporal bien larga"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d, se esperaba 403", rec.Code)
	}
	if p := problemaHTTP(t, rec); !strings.Contains(p.Detail, "superadmin") {
		t.Errorf("detail = %q; tiene que decir a quien pedirselo", p.Detail)
	}
}

func TestAsignarseLaContrasenaAUnoMismoEs409(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	actor := &rbac.Actor{ID: ana.ID, Permissions: todosLosPermisos}

	rec := pedir(t, m, actor, http.MethodPost, "/api/v1/users/"+ana.ID+"/password", `{"password":"una temporal bien larga"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado = %d, se esperaba 409", rec.Code)
	}
	if repo.porEmail[emailDePrueba].usuario.MustChangePassword {
		t.Error("se la asigno igual")
	}
}

func TestAsignarUnaContrasenaCortaNombraElCampo(t *testing.T) {
	m, _, ana := anaEnElModulo(t)
	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost, "/api/v1/users/"+ana.ID+"/password", `{"password":"corta"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d", rec.Code)
	}
	if p := problemaHTTP(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "password" {
		t.Errorf("errors = %+v", p.Errors)
	}
}

func TestAsignarAUnaCuentaQueNoExisteEs404(t *testing.T) {
	m, _ := moduloDePrueba(t)
	rec := pedir(t, m, con(todosLosPermisos...), http.MethodPost,
		"/api/v1/users/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c/password", `{"password":"una temporal bien larga"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("estado = %d, se esperaba 404", rec.Code)
	}
}

func TestLasSolicitudesTraenCorreoYNombre(t *testing.T) {
	m, _, _ := anaEnElModulo(t)
	_ = m.svc.PedirContrasena(context.Background(), emailDePrueba, "", "")

	rec := pedir(t, m, con("identity.user.password"), http.MethodGet, "/api/v1/password-resets", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), emailDePrueba) || !strings.Contains(rec.Body.String(), nombreDePrueba) {
		t.Errorf("cuerpo = %s", rec.Body.String())
	}
}

// --- la contrasena temporal ------------------------------------------------------

// Con contrasena temporal, sesion si y permisos no. Es lo que impide que quien
// la asigno, y la conoce, opere el sitio como esa cuenta.
func TestConContrasenaTemporalNoSeResuelvenPermisos(t *testing.T) {
	s, repo := servicio(t, true)
	u := conUsuario(t, s, emailDePrueba)
	repo.porEmail[emailDePrueba].usuario.MustChangePassword = true

	actor, err := s.Actor(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("Actor: %v; la sesion tiene que seguir valiendo para cambiarla", err)
	}
	if len(actor.Permissions) != 0 {
		t.Errorf("permisos = %v; con contrasena temporal no hay ninguno", actor.Permissions)
	}
}

func TestMeDiceQueHayQueCambiarlaYNoOfrecePermisos(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.porEmail[emailDePrueba].usuario.MustChangePassword = true

	rec := pedir(t, m, &rbac.Actor{ID: ana.ID}, http.MethodGet, "/api/v1/auth/me", "")

	cuerpo := rec.Body.String()
	if !strings.Contains(cuerpo, `"mustChangePassword":true`) || !strings.Contains(cuerpo, `"permissions":[]`) {
		t.Errorf("me = %s", cuerpo)
	}
}

// --- cambiar la propia -----------------------------------------------------------

func cambiar(t *testing.T, m *Module, id, actual, nueva string) (int, []*http.Cookie, string) {
	t.Helper()
	rec := pedir(t, m, &rbac.Actor{ID: id}, http.MethodPost, "/api/v1/auth/password",
		`{"currentPassword":"`+actual+`","newPassword":"`+nueva+`"}`)
	return rec.Code, rec.Result().Cookies(), rec.Body.String()
}

func TestCambiarLaPropiaQuitaLaTemporalYDejaSoloEstaSesion(t *testing.T) {
	m, repo, ana := anaEnElModulo(t)
	repo.porEmail[emailDePrueba].usuario.MustChangePassword = true
	for range 2 {
		if _, err := m.svc.IniciarSesion(context.Background(), IntentoDeSesion{Email: emailDePrueba, Password: claveDePrueba}); err != nil {
			t.Fatalf("IniciarSesion: %v", err)
		}
	}

	code, cookies, cuerpo := cambiar(t, m, ana.ID, claveDePrueba, "la mia nueva y larga")

	if code != http.StatusNoContent {
		t.Fatalf("estado = %d: %s", code, cuerpo)
	}
	if repo.porEmail[emailDePrueba].usuario.MustChangePassword {
		t.Error("sigue marcada como temporal")
	}
	// Las dos de antes revocadas, y una nueva para quien hizo el cambio.
	if n := repo.vivas(ana.ID); n != 1 {
		t.Errorf("sesiones vivas = %d, se esperaba solo la nueva", n)
	}
	puestas := map[string]bool{}
	for _, c := range cookies {
		puestas[c.Name] = true
	}
	for _, nombre := range []string{auth.CookieAT, auth.CookieRT, auth.CookieCSRF} {
		if !puestas[nombre] {
			t.Errorf("no se emitio %s: la sesion que hizo el cambio se quedaria sin refresh", nombre)
		}
	}
	if _, err := m.svc.Autenticar(context.Background(), emailDePrueba, "la mia nueva y larga"); err != nil {
		t.Errorf("no se entra con la nueva: %v", err)
	}
}

func TestCambiarLaPropiaValidaCadaCampo(t *testing.T) {
	casos := []struct {
		nombre, actual, nueva, campo string
	}{
		{"actual mala", "no es la mia de verdad", "la mia nueva y larga", "currentPassword"},
		{"nueva corta", claveDePrueba, "corta", "newPassword"},
		{"nueva igual", claveDePrueba, claveDePrueba, "newPassword"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m, repo, ana := anaEnElModulo(t)
			hash := repo.porEmail[emailDePrueba].hash

			code, cookies, cuerpo := cambiar(t, m, ana.ID, c.actual, c.nueva)

			if code != http.StatusBadRequest || !strings.Contains(cuerpo, `"field":"`+c.campo+`"`) {
				t.Errorf("estado = %d, cuerpo = %s", code, cuerpo)
			}
			if repo.porEmail[emailDePrueba].hash != hash || len(cookies) != 0 {
				t.Error("un cambio rechazado escribio algo o emitio cookies")
			}
		})
	}
}

func TestCambiarLaPropiaSinSesionEs401(t *testing.T) {
	m, _ := moduloDePrueba(t)
	rec := pedir(t, m, nil, http.MethodPost, "/api/v1/auth/password", `{"currentPassword":"x","newPassword":"y"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d, se esperaba 401", rec.Code)
	}
}
