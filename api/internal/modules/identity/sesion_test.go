package identity

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

const passDePrueba = "una-contrasena-larga-de-prueba"

func svcConAna(t *testing.T, dev bool) (*Service, *repoFalso, Usuario) {
	t.Helper()
	svc, repo := servicio(t, dev)

	u, err := svc.Crear(context.Background(), UsuarioNuevo{
		Email:       "ana@casa.com",
		Password:    passDePrueba,
		DisplayName: "Ana",
		Roles:       []string{RolAdmin},
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	return svc, repo, u
}

func TestIniciarSesionEmiteLosTresTokensYLosRoles(t *testing.T) {
	svc, _, u := svcConAna(t, true)

	s, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9", UserAgent: "curl",
	})
	if err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}

	if s.Usuario.ID != u.ID {
		t.Errorf("usuario = %q, se esperaba %q", s.Usuario.ID, u.ID)
	}
	// Los tres, y los tres distintos: reusar uno para otra cosa haria que
	// filtrar el CSRF —que JS puede leer— entregara la sesion.
	if s.AccessToken == "" || s.RefreshToken == "" || s.TokenCSRF == "" {
		t.Fatalf("falta algun token: %+v", s)
	}
	if s.AccessToken == s.RefreshToken || s.RefreshToken == s.TokenCSRF || s.AccessToken == s.TokenCSRF {
		t.Error("dos de los tres tokens son el mismo valor")
	}
}

// El `at` es un token que se puede verificar, no una cadena cualquiera, y lleva
// dentro al usuario con sus roles.
func TestElAccessTokenEmitidoSeVerificaYNombraAlUsuario(t *testing.T) {
	svc, _, u := svcConAna(t, true)

	s, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba,
	})
	if err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}

	c, err := svc.firmante.Verificar(s.AccessToken)
	if err != nil {
		t.Fatalf("el at emitido no se verifica: %v", err)
	}
	if c.Sub != u.ID {
		t.Errorf("sub = %q, se esperaba %q", c.Sub, u.ID)
	}
	if len(c.Roles) != 1 || c.Roles[0] != RolAdmin {
		t.Errorf("roles = %v", c.Roles)
	}
}

// Lo que va a la base es el HASH. Es la mitad de la Decision 007 que se puede
// romper sin que nada falle: guardar el token en claro compila igual y solo se
// nota el dia que se filtra la base.
func TestLoQueSeGuardaEsElHashYNoElRefreshToken(t *testing.T) {
	svc, repo, u := svcConAna(t, true)

	s, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9", UserAgent: "curl/8",
	})
	if err != nil {
		t.Fatalf("IniciarSesion: %v", err)
	}

	if len(repo.sesiones) != 1 {
		t.Fatalf("sesiones guardadas = %d, se esperaba 1", len(repo.sesiones))
	}
	fila := repo.sesiones[0]

	if string(fila.TokenHash) == s.RefreshToken {
		t.Fatal("se guardo el refresh token en claro")
	}
	if !bytes.Equal(fila.TokenHash, auth.HashDeRefresh(s.RefreshToken)) {
		t.Error("lo guardado no es el SHA-256 del token que se entrego")
	}
	if fila.UserID != u.ID {
		t.Errorf("user_id = %q", fila.UserID)
	}
	// La IP y el agente son lo que despues permite mirar las sesiones abiertas
	// y reconocer la que no es tuya.
	if fila.IP != "203.0.113.9" || fila.UserAgent != "curl/8" {
		t.Errorf("no se guardo el contexto de la peticion: %+v", fila)
	}
	if fila.ExpiraEn.IsZero() {
		t.Error("la sesion se guardo sin caducidad")
	}
}

// Los tres casos responden IDENTICO. No basta con comparar el codigo: un
// `detail` distinto delata igual que correos estan registrados, asi que se
// comparan los Problem enteros.
func TestElCorreoQueNoExisteLaPassMalaYLaCuentaApagadaRespondenIgual(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)

	_, errNoExiste := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "nadie@casa.com", Password: passDePrueba,
	})
	_, errPassMala := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: "otra-contrasena-larga-distinta",
	})

	repo.porEmail["ana@casa.com"].usuario.Enabled = false
	_, errApagada := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba,
	})

	var a, b, c *httpx.Problem
	for nombre, par := range map[string]struct {
		err error
		dst **httpx.Problem
	}{"no existe": {errNoExiste, &a}, "pass mala": {errPassMala, &b}, "apagada": {errApagada, &c}} {
		if !errors.As(par.err, par.dst) {
			t.Fatalf("%s: err = %v, se esperaba un *httpx.Problem", nombre, par.err)
		}
	}

	// DeepEqual y no una comparacion campo a campo: lo que hay que afirmar es
	// que NO hay ninguna diferencia, incluidas las que se agreguen manana.
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(b, c) {
		t.Errorf("los tres 401 no son identicos:\n no existe: %+v\n pass mala: %+v\n apagada:   %+v", a, b, c)
	}
	if a.Status != http.StatusUnauthorized {
		t.Errorf("estado = %d, se esperaba 401", a.Status)
	}
}

// Un login fallido no puede dejar rastro de sesion. Si lo dejara, cada intento
// contra un correo real llenaria refresh_tokens desde fuera.
func TestUnLoginFallidoNoGuardaNingunaSesion(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)

	_, _ = svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: "la-que-no-es-larga-igual",
	})

	if len(repo.sesiones) != 0 {
		t.Errorf("un intento fallido dejo %d sesiones", len(repo.sesiones))
	}
}

// El limite corta al sexto, y responde 429 con su espera.
func TestAlSextoIntentoFallidoResponde429(t *testing.T) {
	svc, _, _ := svcConAna(t, true)

	malo := IntentoDeSesion{Email: "ana@casa.com", Password: "no-es-la-contrasena", IP: "203.0.113.9"}

	for n := 1; n <= auth.MaxIntentos; n++ {
		_, err := svc.IniciarSesion(context.Background(), malo)
		var p *httpx.Problem
		if !errors.As(err, &p) || p.Status != http.StatusUnauthorized {
			t.Fatalf("el intento %d dio %v, se esperaba 401", n, err)
		}
	}

	_, err := svc.IniciarSesion(context.Background(), malo)
	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusTooManyRequests {
		t.Fatalf("el intento %d dio %v, se esperaba 429", auth.MaxIntentos+1, err)
	}
	if p.RetryAfter <= 0 {
		t.Error("el 429 no dice cuanto esperar")
	}
}

// Bloqueado, NI SIQUIERA se consulta al usuario: el limite existe para negarle
// al atacante el trabajo de argon2, no solo para negarle la respuesta. Seis
// peticiones por segundo con un correo inventado bastan para ocupar el proceso.
//
// Se cuentan las llamadas al repositorio, y esa es la unica forma de verlo: si
// el limite corriera DESPUES de autenticar, la respuesta seguiria siendo un 429
// y una prueba que solo mire el codigo pasaria con el orden invertido. Lo
// comprobo una mutacion deliberada, que sobrevivio a la primera version de esta
// prueba.
func TestBloqueadoNiSiquieraSeMiraLaContrasena(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)

	for n := 0; n < auth.MaxIntentos; n++ {
		_, _ = svc.IniciarSesion(context.Background(), IntentoDeSesion{
			Email: "ana@casa.com", Password: "no-es-la-contrasena", IP: "203.0.113.9",
		})
	}

	consultasAntes := repo.vecesQueSeAutentico

	_, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9",
	})

	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusTooManyRequests {
		t.Fatalf("err = %v; con el limite agotado ni la contrasena buena entra", err)
	}
	if repo.vecesQueSeAutentico != consultasAntes {
		t.Errorf("se consulto al usuario %d vez/veces estando bloqueado: el limite corre despues de argon2",
			repo.vecesQueSeAutentico-consultasAntes)
	}
	if len(repo.sesiones) != 0 {
		t.Error("se abrio una sesion con el limite agotado")
	}
}

// Alternar mayusculas no da intentos infinitos. La base compara con citext y no
// lo necesita, pero el contador es un mapa de Go: sin normalizar, Ana@casa.com y
// ana@casa.com son dos cajones distintos contra la misma cuenta.
func TestAlternarMayusculasNoRegalaIntentos(t *testing.T) {
	svc, _, _ := svcConAna(t, true)

	correos := []string{"ana@casa.com", "Ana@casa.com", "ANA@CASA.COM", "aNa@CaSa.CoM", "ana@Casa.com"}
	for _, correo := range correos {
		_, _ = svc.IniciarSesion(context.Background(), IntentoDeSesion{
			Email: correo, Password: "no-es-la-contrasena",
		})
	}

	_, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "AnA@casa.com", Password: "no-es-la-contrasena",
	})

	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusTooManyRequests {
		t.Fatalf("err = %v; cinco fallos son cinco aunque cambien las mayusculas", err)
	}
}

// Un error de la base NO cuenta como intento fallido: si contara, una caida de
// Postgres dejaria a todo el mundo bloqueado quince minutos despues de volver.
func TestUnErrorDeLaBaseNoGastaIntentos(t *testing.T) {
	svc, repo, _ := svcConAna(t, true)
	repo.errAlGuardarRefresh = errors.New("la base no contesta")

	bueno := IntentoDeSesion{Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9"}

	for n := 0; n <= auth.MaxIntentos; n++ {
		_, err := svc.IniciarSesion(context.Background(), bueno)
		var p *httpx.Problem
		if errors.As(err, &p) && p.Status == http.StatusTooManyRequests {
			t.Fatalf("en la vuelta %d el fallo de la base se conto como intento", n)
		}
	}
}

// Entrar bien limpia lo que se llevaba fallado.
func TestEntrarBienDespejaElContador(t *testing.T) {
	svc, _, _ := svcConAna(t, true)

	for n := 0; n < auth.MaxIntentos-1; n++ {
		_, _ = svc.IniciarSesion(context.Background(), IntentoDeSesion{
			Email: "ana@casa.com", Password: "no-es-la-contrasena", IP: "203.0.113.9",
		})
	}

	if _, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "ana@casa.com", Password: passDePrueba, IP: "203.0.113.9",
	}); err != nil {
		t.Fatalf("el quinto intento, con la contrasena buena, fallo: %v", err)
	}

	// Y despues quedan los cinco otra vez.
	for n := 1; n <= auth.MaxIntentos; n++ {
		_, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
			Email: "ana@casa.com", Password: "no-es-la-contrasena", IP: "203.0.113.9",
		})
		var p *httpx.Problem
		if errors.As(err, &p) && p.Status == http.StatusTooManyRequests {
			t.Fatalf("el intento %d se bloqueo: entrar bien no limpio el contador", n)
		}
	}
}

// La cuenta que siembra `cmd/seed` no entra fuera de desarrollo, y la marca
// viaja en la fila: sigue valiendo si la base acaba copiada a otro entorno.
func TestLaCuentaSembradaNoIniciaSesionFueraDeDesarrollo(t *testing.T) {
	// Se siembra EN desarrollo y despues se cambia el entorno, que es justo el
	// caso que la marca protege: la base sembrada en la maquina de alguien y
	// copiada a otro entorno. Sembrar directamente con dev=false ni siquiera
	// esta permitido, asi que probaria otra cosa.
	svc, repo := servicio(t, true)

	if _, err := svc.SembrarSuperadminDeDesarrollo(context.Background(), UsuarioNuevo{
		Email: "seed@casa.com", Password: passDePrueba, DisplayName: "Seed",
		Roles: []string{RolSuperadmin},
	}); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	svc.dev = false

	_, err := svc.IniciarSesion(context.Background(), IntentoDeSesion{
		Email: "seed@casa.com", Password: passDePrueba,
	})

	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, se esperaba 401", err)
	}
	if len(repo.sesiones) != 0 {
		t.Error("se abrio sesion con la cuenta sembrada fuera de desarrollo")
	}
}

// Perfil devuelve los permisos EFECTIVOS, resueltos por los roles: es lo que
// deja al frontend ocultar lo que no aplica sin reimplementar nada.
func TestPerfilDevuelveRolesYPermisosResueltos(t *testing.T) {
	svc, _, u := svcConAna(t, true)

	perfil, err := svc.Perfil(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("Perfil: %v", err)
	}

	if perfil.Usuario.Email != "ana@casa.com" {
		t.Errorf("email = %q", perfil.Usuario.Email)
	}
	if len(perfil.Roles) != 1 || perfil.Roles[0] != RolAdmin {
		t.Errorf("roles = %v", perfil.Roles)
	}
}
