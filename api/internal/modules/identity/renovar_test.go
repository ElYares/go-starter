package identity

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

func hashDe(token string) []byte { return auth.HashDeRefresh(token) }

func entrarComoAna(t *testing.T, svc *Service) Sesion {
	t.Helper()
	s, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9",
	})
	if err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}
	return s
}

func renovar(svc *Service, rt string) (Sesion, error) {
	return svc.Renovar(context.Background(), RenovacionDeSesion{RefreshToken: rt, IP: "203.0.113.9", UserAgent: "curl/8"})
}

func es401(t *testing.T, err error) *httpx.Problem {
	t.Helper()
	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, se esperaba un 401", err)
	}
	return p
}

// El flujo principal: el `rt` se cambia por un par nuevo, y la fila anterior
// queda apuntando a la nueva. Ese enlace es lo que despues delata un reuso.
func TestRenovarEmiteUnParNuevoYEncadenaElAnterior(t *testing.T) {
	svc, repo, u := svcConAna(t, true)
	login := entrarComoAna(t, svc)

	nueva, err := renovar(svc, login.RefreshToken)
	if err != nil {
		t.Fatalf("Renovar: %v", err)
	}

	// Solo el `rt` tiene que cambiar. El `at` es un JWT determinista —sub,
	// roles y exp en segundos— y dos emitidos en el mismo segundo son identicos
	// byte a byte; exigir que difiera hace fallar la prueba cuando corre rapido.
	if nueva.RefreshToken == login.RefreshToken {
		t.Error("el refresh devolvio el mismo rt")
	}
	if nueva.AccessToken == "" {
		t.Error("el refresh no emitio at")
	}
	// CU-002 paso 5: el CSRF tambien rota.
	if nueva.TokenCSRF == "" || nueva.TokenCSRF == login.TokenCSRF {
		t.Error("el refresh no roto el token CSRF")
	}
	if nueva.Usuario.ID != u.ID {
		t.Errorf("usuario = %q", nueva.Usuario.ID)
	}

	anterior, sucesora := repo.filaDe(login.RefreshToken), repo.filaDe(nueva.RefreshToken)
	if sucesora == nil {
		t.Fatal("la sesion nueva no quedo guardada con el hash del token entregado")
	}
	if anterior.guardado.ReemplazadoPor == nil || *anterior.guardado.ReemplazadoPor != sucesora.guardado.ID {
		t.Errorf("replaced_by = %v; tiene que apuntar a la sucesora %s", anterior.guardado.ReemplazadoPor, sucesora.guardado.ID)
	}
	if bytes.Equal(sucesora.hash, []byte(nueva.RefreshToken)) {
		t.Error("la sucesora se guardo con el token en claro")
	}
	if repo.vivas(u.ID) != 1 {
		t.Errorf("sesiones vivas = %d; exactamente una por sesion", repo.vivas(u.ID))
	}
}

// E2: un `rt` que ya se roto solo llega si hay otra copia. Se revocan TODAS
// las sesiones de la persona, no solo esa cadena: no hay forma de saber cual
// de las copias es la legitima, ni que abrio el ladron con ella.
func TestUnRefreshReusadoRevocaTodasLasSesiones(t *testing.T) {
	svc, repo, u := svcConAna(t, true)
	enElPortatil := entrarComoAna(t, svc)
	enElTelefono := entrarComoAna(t, svc)

	delLadron, err := renovar(svc, enElPortatil.RefreshToken)
	if err != nil {
		t.Fatalf("primera renovacion: %v", err)
	}

	_, err = renovar(svc, enElPortatil.RefreshToken)
	es401(t, err)

	if repo.vivas(u.ID) != 0 {
		t.Errorf("quedaron %d sesiones vivas despues de detectar el reuso", repo.vivas(u.ID))
	}
	for nombre, rt := range map[string]string{"la del ladron": delLadron.RefreshToken, "la del telefono": enElTelefono.RefreshToken} {
		if _, err := renovar(svc, rt); err == nil {
			t.Errorf("%s siguio renovando despues del reuso", nombre)
		}
	}
}

// Un robo se detecta UNA vez. Despues, el token viejo esta revocado como todo
// lo demas, y repetirlo no puede volver a tumbar las sesiones que la persona
// abra al entrar de nuevo. Si pudiera, un token robado seria una forma de
// echar a alguien para siempre sin saber su contrasena.
func TestUnTokenRobadoYaDetectadoNoVuelveACerrarLasSesionesNuevas(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	robado := entrarComoAna(t, svc)
	if _, err := renovar(svc, robado.RefreshToken); err != nil {
		t.Fatalf("Renovar: %v", err)
	}
	_, err := renovar(svc, robado.RefreshToken)
	es401(t, err)

	deNuevo := entrarComoAna(t, svc)

	_, err = renovar(svc, robado.RefreshToken)
	es401(t, err)

	if repo.vecesQueSeRevocoTodo != 1 {
		t.Errorf("revocaciones generales = %d; el replay de un robo ya detectado no revoca otra vez", repo.vecesQueSeRevocoTodo)
	}
	if _, err := renovar(svc, deNuevo.RefreshToken); err != nil {
		t.Errorf("la sesion abierta despues de la deteccion quedo cerrada por el replay: %v", err)
	}
}

// Los rechazos son indistinguibles. Decirle a quien presenta un token robado
// que se detecto el robo le ahorra la duda.
func TestLosRechazosDelRefreshSonIdenticos(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)

	reusado := entrarComoAna(t, svc)
	if _, err := renovar(svc, reusado.RefreshToken); err != nil {
		t.Fatalf("Renovar: %v", err)
	}
	caducado := entrarComoAna(t, svc)
	repo.filaDe(caducado.RefreshToken).guardado.ExpiraEn = time.Now().Add(-time.Minute)
	revocado := entrarComoAna(t, svc)
	if err := svc.CerrarSesion(context.Background(), revocado.RefreshToken); err != nil {
		t.Fatalf("CerrarSesion: %v", err)
	}

	var problemas []*httpx.Problem
	for _, rt := range []string{"", "no-existe", caducado.RefreshToken, revocado.RefreshToken, reusado.RefreshToken} {
		_, err := renovar(svc, rt)
		problemas = append(problemas, es401(t, err))
	}
	for i := 1; i < len(problemas); i++ {
		if !reflect.DeepEqual(problemas[0], problemas[i]) {
			t.Errorf("rechazo %d distinto del primero:\n %+v\n %+v", i, problemas[0], problemas[i])
		}
	}
}

// Un token caducado o revocado se rechaza SIN tumbar las demas sesiones: no es
// un robo, es una sesion que termino.
func TestCaducadoORevocadoNoRevocanLasDemasSesiones(t *testing.T) {
	svc, repo, u := svcConAna(t, true)
	otra := entrarComoAna(t, svc)
	caducado := entrarComoAna(t, svc)
	repo.filaDe(caducado.RefreshToken).guardado.ExpiraEn = time.Now().Add(-time.Second)

	_, err := renovar(svc, caducado.RefreshToken)
	es401(t, err)

	if repo.vecesQueSeRevocoTodo != 0 {
		t.Error("un token caducado disparo la revocacion general")
	}
	// Se rechaza antes de firmar nada. El WHERE del compare-and-set tambien lo
	// pararia, y por eso una mutacion que quitaba este chequeo sobrevivio: la
	// respuesta seguia siendo 401, pero despues de emitir tokens y abrir una
	// transaccion para tirarla.
	if repo.vecesQueSeIntentoRotar != 0 {
		t.Error("un token caducado llego a intentar la rotacion")
	}
	if repo.filaDe(otra.RefreshToken).guardado.RevocadoEn != nil {
		t.Error("la otra sesion quedo revocada")
	}
	_ = u
}

// CU-002 criterio 6: despues de un logout, el `rt` anterior no renueva.
func TestTrasCerrarSesionElRefreshAnteriorNoRenueva(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	s := entrarComoAna(t, svc)
	enOtroLado := entrarComoAna(t, svc)

	if err := svc.CerrarSesion(context.Background(), s.RefreshToken); err != nil {
		t.Fatalf("CerrarSesion: %v", err)
	}

	_, err := renovar(svc, s.RefreshToken)
	es401(t, err)

	// Cerrar en el portatil no saca a la persona del telefono.
	if _, err := renovar(svc, enOtroLado.RefreshToken); err != nil {
		t.Errorf("el logout de una sesion cerro tambien la otra: %v", err)
	}
	if repo.vecesQueSeRevocoTodo != 0 {
		t.Error("usar un token de una sesion cerrada se trato como robo")
	}
}

// Deshabilitar la cuenta corta la sesion en el siguiente refresh, no dentro de
// catorce dias.
func TestUnaCuentaDeshabilitadaNoRenueva(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	s := entrarComoAna(t, svc)

	repo.porEmail["ana@casa.com"].usuario.Enabled = false

	_, err := renovar(svc, s.RefreshToken)
	es401(t, err)
}

// La cuenta sembrada no renueva fuera de desarrollo, igual que no entra.
func TestLaCuentaSembradaNoRenuevaFueraDeDesarrollo(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	s := entrarComoAna(t, svc)

	repo.porEmail["ana@casa.com"].usuario.DevSeed = true
	svc.dev = false

	_, err := renovar(svc, s.RefreshToken)
	es401(t, err)
}

// La ventana entre leer la fila y rotarla: otra peticion la roto primero. El
// compare-and-set no afecta nada, y el service tiene que tratarlo como lo que
// es, un reuso simultaneo.
func TestSiOtraPeticionRotaEntreLeerYEscribirEsRobo(t *testing.T) {
	svc, repo, u := svcConAna(t, true)
	s := entrarComoAna(t, svc)

	repo.antesDeRotar = func() {
		repo.antesDeRotar = nil
		if _, err := renovar(svc, s.RefreshToken); err != nil {
			t.Fatalf("la renovacion que gana la carrera fallo: %v", err)
		}
	}

	_, err := renovar(svc, s.RefreshToken)
	es401(t, err)

	if repo.vecesQueSeRevocoTodo != 1 {
		t.Errorf("revocaciones generales = %d; perder la carrera contra otra rotacion es un reuso", repo.vecesQueSeRevocoTodo)
	}
	if repo.vivas(u.ID) != 0 {
		t.Errorf("quedaron %d sesiones vivas", repo.vivas(u.ID))
	}
}

// Y si lo que gano la carrera fue un logout, no es robo: la persona cerro la
// sesion y renovarla no la resucita, pero tampoco hay que tumbar las demas.
func TestSiUnLogoutGanaLaCarreraNoEsRobo(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	s := entrarComoAna(t, svc)
	otra := entrarComoAna(t, svc)

	repo.antesDeRotar = func() {
		repo.antesDeRotar = nil
		_ = svc.CerrarSesion(context.Background(), s.RefreshToken)
	}

	_, err := renovar(svc, s.RefreshToken)
	es401(t, err)

	if repo.vecesQueSeRevocoTodo != 0 {
		t.Error("un logout concurrente se trato como robo")
	}
	if repo.filaDe(otra.RefreshToken).guardado.RevocadoEn != nil {
		t.Error("la otra sesion quedo revocada")
	}
}

func TestCerrarSesionSinTokenNoEsUnError(t *testing.T) {
	svc, _, _ := svcConAna(t, true)
	if err := svc.CerrarSesion(context.Background(), ""); err != nil {
		t.Errorf("CerrarSesion sin token: %v", err)
	}
	if err := svc.CerrarSesion(context.Background(), "no-existe"); err != nil {
		t.Errorf("CerrarSesion con un token desconocido: %v", err)
	}
}

// --- HTTP ---

func pedirConRefresh(t *testing.T, m *Module, ruta, rt string) *httptest.ResponseRecorder {
	t.Helper()
	r := httpx.NewRouter()
	m.Routes(r)

	req := httptest.NewRequest(http.MethodPost, ruta, nil)
	req.Header.Set(auth.CabeceraCSRF, "el-token")
	if rt != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieRT, Value: rt})
	}
	rec := httptest.NewRecorder()
	observ.Chain(r.Handler(), observ.TraceID).ServeHTTP(rec, req)
	return rec
}

func cookiesPor(rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		out[c.Name] = c
	}
	return out
}

var lasCuatro = []string{auth.CookieAT, auth.CookieRT, auth.CookieCSRF, auth.CookieHasSession}

func TestElRefreshResponde204YLasCuatroCookiesNuevas(t *testing.T) {
	m, _ := moduloDePrueba(t)
	crearAna(t, m)
	s := entrarComoAna(t, m.svc)

	rec := pedirConRefresh(t, m, "/api/v1/auth/refresh", s.RefreshToken)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	puestas := cookiesPor(rec)
	for _, n := range lasCuatro {
		if c, ok := puestas[n]; !ok || c.MaxAge < 0 {
			t.Errorf("la cookie %s no se reemitio", n)
		}
	}
	if puestas[auth.CookieRT].Value == s.RefreshToken {
		t.Error("la cookie rt lleva el token viejo")
	}
}

// E1: sin sesion que renovar se borran las cuatro, incluida la pista. Si la
// pista quedara, cada carga pediria un refresh condenado a fallar.
func TestUnRefreshRechazadoBorraLasCuatroCookies(t *testing.T) {
	for nombre, rt := range map[string]string{"sin cookie": "", "desconocido": "no-existe"} {
		t.Run(nombre, func(t *testing.T) {
			m, _ := moduloDePrueba(t)

			rec := pedirConRefresh(t, m, "/api/v1/auth/refresh", rt)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("estado = %d, se esperaba 401", rec.Code)
			}
			puestas := cookiesPor(rec)
			for _, n := range lasCuatro {
				if c, ok := puestas[n]; !ok || c.MaxAge >= 0 {
					t.Errorf("la cookie %s no se borro", n)
				}
			}
		})
	}
}

func TestElLogoutRevocaYBorraLasCookies(t *testing.T) {
	m, _ := moduloDePrueba(t)
	crearAna(t, m)
	s := entrarComoAna(t, m.svc)

	rec := pedirConRefresh(t, m, "/api/v1/auth/logout", s.RefreshToken)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	puestas := cookiesPor(rec)
	for _, n := range lasCuatro {
		if c, ok := puestas[n]; !ok || c.MaxAge >= 0 {
			t.Errorf("la cookie %s no se borro", n)
		}
	}
	if _, err := renovar(m.svc, s.RefreshToken); err == nil {
		t.Error("el rt siguio renovando despues del logout")
	}
}

// Cerrar una sesion que no existe es haberla cerrado: 204 igual.
func TestElLogoutSinCookieResponde204(t *testing.T) {
	m, _ := moduloDePrueba(t)

	if rec := pedirConRefresh(t, m, "/api/v1/auth/logout", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("estado = %d, se esperaba 204", rec.Code)
	}
}

func crearAna(t *testing.T, m *Module) {
	t.Helper()
	if _, err := m.svc.Crear(context.Background(), UsuarioNuevo{
		Email: "ana@casa.com", Password: passDePrueba, DisplayName: "Ana", Roles: []string{RolAdmin},
	}); err != nil {
		t.Fatalf("Crear: %v", err)
	}
}
