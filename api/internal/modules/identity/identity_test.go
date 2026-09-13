package identity

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Estas pruebas corren sin base. Lo que comprueban es lo que decide el service:
// que se valida, que se traduce a que codigo, y quien entra.
//
// La invariante del ultimo superadmin NO se puede probar aqui de verdad: vive en el
// WHERE de la sentencia que escribe, que es el unico sitio donde es atomica.
// Aqui solo se comprueba que su sentinela sale como 409. Que la sentencia la
// aplique de verdad lo prueba repo_integracion_test.go.

type filaFalsa struct {
	usuario Usuario
	hash    string
}

type repoFalso struct {
	porEmail map[string]*filaFalsa
	roles    map[string][]string

	// permisos, si esta puesto, sustituye a la lista fija. Ver permisosDe.
	permisos *[]string

	// vecesQueSeAutentico es lo que permite afirmar que el limite corre ANTES
	// de tocar argon2. Sin contar las llamadas, un limite mal colocado responde
	// 429 igual —solo que despues de gastar la CPU— y la prueba no lo nota.
	vecesQueSeAutentico int

	// Las sesiones guardadas. Se recuerdan enteras y no solo se cuentan: lo que
	// hay que poder afirmar es que ahi dentro va el HASH y no el token.
	sesiones []SesionNueva

	// Lo que el repositorio de verdad devolveria; aqui se pone a mano para
	// comprobar la traduccion.
	errAlQuitarRol      error
	errAlDeshabilitar   error
	errAlGuardarRefresh error

	// refresh son las filas de `refresh_tokens`, por id, con su hash. Imitan el
	// estado que el refresh lee y escribe; ver refresh_falso_test.go.
	refresh map[string]*filaRefresh
	// antesDeRotar deja simular que otra peticion cambio la fila entre la
	// lectura del service y su compare-and-set, que es justo la ventana que el
	// WHERE del update de verdad cierra.
	antesDeRotar func()
	// vecesQueSeRevocoTodo cuenta las revocaciones generales: es la unica forma
	// de distinguir "se detecto robo" de "solo se rechazo".
	vecesQueSeRevocoTodo int
	// vecesQueSeIntentoRotar distingue "rechazado antes de gastar nada" de
	// "rechazado por el compare-and-set despues de firmar tokens y abrir una
	// transaccion". El resultado HTTP es el mismo; el trabajo no.
	vecesQueSeIntentoRotar int
}

func (r *repoFalso) guardarRefresh(_ context.Context, s SesionNueva) error {
	if r.errAlGuardarRefresh != nil {
		return r.errAlGuardarRefresh
	}
	r.sesiones = append(r.sesiones, s)
	r.registrarRefresh(s)
	return nil
}

func nuevoRepoFalso() *repoFalso {
	return &repoFalso{porEmail: map[string]*filaFalsa{}, roles: map[string][]string{}, refresh: map[string]*filaRefresh{}}
}

func (r *repoFalso) crear(_ context.Context, u Usuario, hash string) (Usuario, error) {
	if _, ya := r.porEmail[strings.ToLower(u.Email)]; ya {
		return Usuario{}, errYaExiste
	}
	u.Version = 1
	r.porEmail[strings.ToLower(u.Email)] = &filaFalsa{usuario: u, hash: hash}
	return u, nil
}

func (r *repoFalso) porID(_ context.Context, id string) (Usuario, error) {
	for _, f := range r.porEmail {
		if f.usuario.ID == id {
			return f.usuario, nil
		}
	}
	return Usuario{}, errNoExiste
}

// La busqueda es insensible a mayusculas porque en la base la hace `citext`. Un
// doble que compara sensible daria por buena una normalizacion que la base no
// necesita, y escondería el bug al reves.
func (r *repoFalso) paraAutenticar(_ context.Context, email string) (Usuario, string, error) {
	r.vecesQueSeAutentico++
	f, ok := r.porEmail[strings.ToLower(email)]
	if !ok {
		return Usuario{}, "", errNoExiste
	}
	return f.usuario, f.hash, nil
}

func (r *repoFalso) actualizarCredencial(_ context.Context, email, hash, nombre string) (Usuario, error) {
	f, ok := r.porEmail[strings.ToLower(email)]
	if !ok {
		return Usuario{}, errNoExiste
	}
	f.hash = hash
	f.usuario.DisplayName = nombre
	f.usuario.Enabled = true
	f.usuario.DevSeed = true
	f.usuario.Version++
	return f.usuario, nil
}

// permisosDe devuelve una lista fija salvo que la prueba pida otra cosa. El
// puntero a nil de `permisos` significa "los de siempre"; una lista vacia
// declarada significa "este usuario no tiene ninguno", que es un caso distinto
// y hay que poder pedirlo.
func (r *repoFalso) permisosDe(_ context.Context, userID string) ([]string, error) {
	if r.permisos != nil {
		return *r.permisos, nil
	}
	return []string{"identity.user.read", "settings.read"}, nil
}

func (r *repoFalso) rolesDe(_ context.Context, userID string) ([]string, error) {
	return slices.Clone(r.roles[userID]), nil
}

func (r *repoFalso) asignarRol(_ context.Context, userID, rol string) error {
	if rol != RolSuperadmin && rol != RolAdmin && rol != RolStaff && rol != RolViewer {
		return errRolNoExiste
	}
	if !slices.Contains(r.roles[userID], rol) {
		r.roles[userID] = append(r.roles[userID], rol)
	}
	return nil
}

func (r *repoFalso) quitarRol(context.Context, string, string) error { return r.errAlQuitarRol }

func (r *repoFalso) deshabilitar(_ context.Context, userID string) (Usuario, error) {
	if r.errAlDeshabilitar != nil {
		return Usuario{}, r.errAlDeshabilitar
	}
	u, err := r.porID(context.Background(), userID)
	if err != nil {
		return Usuario{}, err
	}
	u.Enabled = false
	return u, nil
}

func servicio(t *testing.T, dev bool) (*Service, *repoFalso) {
	t.Helper()
	repo := nuevoRepoFalso()

	// El firmante y el contador van de verdad, no falsos: son baratos, no tocan
	// nada de fuera, y lo que interesa comprobar de IniciarSesion es justo como
	// los usa —el orden del limite y que lo que se guarda es el hash.
	f, err := auth.NewFirmante("llave-de-prueba-de-identity")
	if err != nil {
		t.Fatalf("NewFirmante: %v", err)
	}

	return &Service{repo: repo, dev: dev, firmante: f, intentos: auth.NewIntentos()}, repo
}

const (
	emailDePrueba  = "ana@casa.com"
	claveDePrueba  = "una clave larga de verdad"
	nombreDePrueba = "Ana"
)

func conUsuario(t *testing.T, s *Service, email string) Usuario {
	t.Helper()
	u, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: email, Password: claveDePrueba, DisplayName: nombreDePrueba, Roles: []string{RolStaff},
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	return u
}

func problemaDe(t *testing.T, err error) *httpx.Problem {
	t.Helper()
	var p *httpx.Problem
	if !errors.As(err, &p) {
		t.Fatalf("el error no es un *httpx.Problem, es %T: %v", err, err)
	}
	return p
}

func TestCrearGuardaUnHashYNoLaContrasena(t *testing.T) {
	s, repo := servicio(t, true)
	conUsuario(t, s, emailDePrueba)

	guardado := repo.porEmail[emailDePrueba].hash
	if guardado == claveDePrueba {
		t.Fatal("se guardo la contrasena tal cual")
	}
	if strings.Contains(guardado, claveDePrueba) {
		t.Fatalf("lo guardado contiene la contrasena: %q", guardado)
	}
	if !strings.HasPrefix(guardado, "$argon2id$") {
		t.Errorf("lo guardado no es un argon2id: %q", guardado)
	}
}

func TestElIdDeUnUsuarioEsUnUuidV7(t *testing.T) {
	s, _ := servicio(t, true)
	u := conUsuario(t, s, emailDePrueba)

	id, err := uuid.Parse(u.ID)
	if err != nil {
		t.Fatalf("el id no es un uuid: %q", u.ID)
	}
	if id.Version() != 7 {
		t.Errorf("version del uuid = %d, se esperaba 7", id.Version())
	}
}

// El correo NO se baja a minusculas en Go: quien garantiza que dos capitalizaciones
// son la misma cuenta es el `citext` de la columna. Si el service normalizara,
// esa garantia dejaria de ejercitarse y el dia que alguien escriba una consulta
// que se la salta, nadie se enteraria.
func TestElCorreoSeGuardaComoLoEscribieronSoloRecortado(t *testing.T) {
	s, _ := servicio(t, true)

	u, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: "  Ana@Casa.com  ", Password: claveDePrueba, DisplayName: nombreDePrueba,
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	if u.Email != "Ana@Casa.com" {
		t.Errorf("email = %q, se esperaba %q", u.Email, "Ana@Casa.com")
	}
}

func TestCrearValidaAntesDeTocarLaBase(t *testing.T) {
	casos := []struct {
		nombre string
		nuevo  UsuarioNuevo
		campo  string
		codigo string
	}{
		{"sin correo", UsuarioNuevo{Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "required"},
		{"correo sin arroba", UsuarioNuevo{Email: "ana", Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "format"},
		{"correo con dos arrobas", UsuarioNuevo{Email: "a@b@c.com", Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "format"},
		{"dominio sin punto", UsuarioNuevo{Email: "ana@casa", Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "format"},
		{"correo con espacio", UsuarioNuevo{Email: "an a@casa.com", Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "format"},
		{"correo larguisimo", UsuarioNuevo{Email: strings.Repeat("a", 250) + "@casa.com", Password: claveDePrueba, DisplayName: nombreDePrueba}, "email", "max"},
		{"sin contrasena", UsuarioNuevo{Email: emailDePrueba, DisplayName: nombreDePrueba}, "password", "required"},
		{"contrasena corta", UsuarioNuevo{Email: emailDePrueba, Password: "corta", DisplayName: nombreDePrueba}, "password", "min"},
		{"contrasena de once", UsuarioNuevo{Email: emailDePrueba, Password: strings.Repeat("x", 11), DisplayName: nombreDePrueba}, "password", "min"},
		{"contrasena enorme", UsuarioNuevo{Email: emailDePrueba, Password: strings.Repeat("x", 1025), DisplayName: nombreDePrueba}, "password", "max"},
		{"sin nombre", UsuarioNuevo{Email: emailDePrueba, Password: claveDePrueba}, "displayName", "required"},
		{"nombre en blanco", UsuarioNuevo{Email: emailDePrueba, Password: claveDePrueba, DisplayName: "   "}, "displayName", "required"},
		{"nombre larguisimo", UsuarioNuevo{Email: emailDePrueba, Password: claveDePrueba, DisplayName: strings.Repeat("n", 121)}, "displayName", "max"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s, repo := servicio(t, true)

			_, err := s.Crear(context.Background(), c.nuevo)
			if err == nil {
				t.Fatal("se acepto un usuario invalido")
			}

			p := problemaDe(t, err)
			if p.Status != http.StatusBadRequest {
				t.Errorf("estado = %d, se esperaba 400", p.Status)
			}
			if len(p.Errors) != 1 || p.Errors[0].Field != c.campo || p.Errors[0].Code != c.codigo {
				t.Errorf("errores = %+v; se esperaba el campo %q con codigo %q", p.Errors, c.campo, c.codigo)
			}
			if len(repo.porEmail) != 0 {
				t.Error("la validacion dejo pasar la escritura: el repositorio recibio algo")
			}
		})
	}
}

// La contrasena de exactamente doce entra. Sin esta, un `<=` en vez de un `<`
// dejaria fuera al que cumple el minimo documentado y nadie lo notaria.
func TestLaContrasenaDelLargoMinimoSeAcepta(t *testing.T) {
	s, _ := servicio(t, true)

	if _, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: strings.Repeat("x", minPassword), DisplayName: nombreDePrueba,
	}); err != nil {
		t.Fatalf("una contrasena de %d caracteres tenia que entrar: %v", minPassword, err)
	}
}

func TestCrearAsignaLosRolesQueLePiden(t *testing.T) {
	s, repo := servicio(t, true)

	u, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
		Roles: []string{RolAdmin, RolViewer},
	})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}

	roles := repo.roles[u.ID]
	slices.Sort(roles)
	if strings.Join(roles, ",") != "admin,viewer" {
		t.Errorf("roles = %v, se esperaban admin y viewer", roles)
	}
}

func TestUnRolQueNoExisteEs409(t *testing.T) {
	s, _ := servicio(t, true)

	_, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
		Roles: []string{"inventado"},
	})
	if err == nil {
		t.Fatal("se acepto un rol que no existe")
	}
	if p := problemaDe(t, err); p.Status != http.StatusConflict {
		t.Errorf("estado = %d, se esperaba 409", p.Status)
	}
}

func TestUnCorreoRepetidoEs409(t *testing.T) {
	s, _ := servicio(t, true)
	conUsuario(t, s, emailDePrueba)

	_, err := s.Crear(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
	})
	if err == nil {
		t.Fatal("se acepto un correo repetido")
	}
	if p := problemaDe(t, err); p.Status != http.StatusConflict {
		t.Errorf("estado = %d, se esperaba 409", p.Status)
	}
}

func TestAutenticarAceptaLaContrasenaCorrecta(t *testing.T) {
	s, _ := servicio(t, true)
	creado := conUsuario(t, s, emailDePrueba)

	u, err := s.Autenticar(context.Background(), emailDePrueba, claveDePrueba)
	if err != nil {
		t.Fatalf("Autenticar: %v", err)
	}
	if u.ID != creado.ID {
		t.Errorf("id = %q, se esperaba %q", u.ID, creado.ID)
	}
}

// La otra capitalizacion tiene que entrar igual: es la misma cuenta. En la base
// lo resuelve `citext`; el doble lo imita para que esta prueba signifique algo.
func TestAutenticarNoDistingueMayusculasEnElCorreo(t *testing.T) {
	s, _ := servicio(t, true)
	conUsuario(t, s, emailDePrueba)

	if _, err := s.Autenticar(context.Background(), "ANA@CASA.COM", claveDePrueba); err != nil {
		t.Fatalf("Autenticar con otra capitalizacion: %v", err)
	}
}

// El corazon del asunto: las tres formas de no entrar responden EXACTAMENTE lo
// mismo. En cuanto una se distinga, probar mil correos y quedarse con los que
// responden distinto da la lista de cuentas registradas.
func TestLasTresFormasDeNoEntrarRespondenIgual(t *testing.T) {
	s, repo := servicio(t, true)
	conUsuario(t, s, emailDePrueba)

	conUsuario(t, s, "sin.acceso@casa.com")
	repo.porEmail["sin.acceso@casa.com"].usuario.Enabled = false

	_, errClaveMala := s.Autenticar(context.Background(), emailDePrueba, "otra clave larga aqui")
	_, errNoExiste := s.Autenticar(context.Background(), "nadie@casa.com", claveDePrueba)
	_, errApagado := s.Autenticar(context.Background(), "sin.acceso@casa.com", claveDePrueba)

	casos := map[string]error{
		"contrasena mala":       errClaveMala,
		"correo que no existe":  errNoExiste,
		"usuario deshabilitado": errApagado,
	}

	// La comparacion es del Problem entero, no solo del codigo: un `detail`
	// distinto —"ese correo no existe"— tambien delata, y solo se ve mirandolo
	// completo.
	referencia := httpx.Unauthorized()
	for nombre, err := range casos {
		if err == nil {
			t.Fatalf("%s: se dejo entrar", nombre)
		}
		if p := problemaDe(t, err); !reflect.DeepEqual(p, referencia) {
			t.Errorf("%s respondio %+v; se esperaba exactamente %+v", nombre, *p, *referencia)
		}
	}
}

// El superadmin sembrado lleva la marca en la fila, no en la maquina del que
// sembro: es lo que hace que un volcado de desarrollo copiado a otro entorno no
// traiga una cuenta con contrasena publica dentro.
func TestElSuperadminDeDesarrolloSoloEntraEnDesarrollo(t *testing.T) {
	for _, dev := range []bool{true, false} {
		s, _ := servicio(t, dev)
		if _, err := s.SembrarSuperadminDeDesarrollo(context.Background(), UsuarioNuevo{
			Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
			Roles: []string{RolSuperadmin},
		}); err != nil {
			if dev {
				t.Fatalf("sembrar en dev: %v", err)
			}
			// Fuera de dev ni siquiera se siembra; el resto de la prueba usa un
			// service en dev para sembrar y otro fuera para autenticar.
			continue
		}

		u, err := s.Autenticar(context.Background(), emailDePrueba, claveDePrueba)
		if dev {
			if err != nil {
				t.Fatalf("en dev la cuenta sembrada tenia que entrar: %v", err)
			}
			if !u.DevSeed {
				t.Error("la cuenta sembrada no quedo marcada como dev_seed")
			}
		}
	}

	// Y la mitad que importa: la MISMA fila, con el entorno cambiado debajo.
	sembrador, repo := servicio(t, true)
	if _, err := sembrador.SembrarSuperadminDeDesarrollo(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
		Roles: []string{RolSuperadmin},
	}); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	produccion := &Service{repo: repo, dev: false}
	if _, err := produccion.Autenticar(context.Background(), emailDePrueba, claveDePrueba); err == nil {
		t.Fatal("el superadmin de desarrollo entro fuera de desarrollo")
	} else if p := problemaDe(t, err); p.Status != http.StatusUnauthorized {
		t.Errorf("estado = %d, se esperaba 401", p.Status)
	}
}

// Un usuario normal no arrastra la marca: si la llevara, nadie podria entrar en
// produccion y el sintoma seria "el login no funciona en ningun lado".
func TestUnUsuarioNormalNoQuedaMarcadoComoDeDesarrollo(t *testing.T) {
	s, _ := servicio(t, false)
	conUsuario(t, s, emailDePrueba)

	u, err := s.Autenticar(context.Background(), emailDePrueba, claveDePrueba)
	if err != nil {
		t.Fatalf("Autenticar: %v", err)
	}
	if u.DevSeed {
		t.Error("un usuario creado por la via normal quedo marcado como dev_seed")
	}
}

func TestSembrarElSuperadminDeDesarrolloNoCorreFueraDeDev(t *testing.T) {
	s, repo := servicio(t, false)

	_, err := s.SembrarSuperadminDeDesarrollo(context.Background(), UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
	})
	if err == nil {
		t.Fatal("se sembro un superadmin de desarrollo fuera de desarrollo")
	}
	if len(repo.porEmail) != 0 {
		t.Error("se escribio igual")
	}
}

// Un seed que falla la segunda vez no sirve: el caso normal es correrlo despues
// de cada cambio de esquema, no una sola vez en la vida del proyecto.
func TestSembrarDosVecesReescribeLaCredencialYConservaElId(t *testing.T) {
	s, repo := servicio(t, true)
	nuevo := UsuarioNuevo{
		Email: emailDePrueba, Password: claveDePrueba, DisplayName: nombreDePrueba,
		Roles: []string{RolAdmin},
	}

	primero, err := s.SembrarSuperadminDeDesarrollo(context.Background(), nuevo)
	if err != nil {
		t.Fatalf("primera siembra: %v", err)
	}
	hashInicial := repo.porEmail[emailDePrueba].hash

	nuevo.Password = "otra clave larga distinta"
	segundo, err := s.SembrarSuperadminDeDesarrollo(context.Background(), nuevo)
	if err != nil {
		t.Fatalf("segunda siembra: %v", err)
	}

	if segundo.ID != primero.ID {
		t.Errorf("el id cambio de %q a %q: las filas que apuntaban al primero quedarian huerfanas", primero.ID, segundo.ID)
	}
	if repo.porEmail[emailDePrueba].hash == hashInicial {
		t.Error("la segunda siembra no reescribio la contrasena")
	}
	if _, err := s.Autenticar(context.Background(), emailDePrueba, "otra clave larga distinta"); err != nil {
		t.Errorf("la contrasena nueva no entra: %v", err)
	}
	if _, err := s.Autenticar(context.Background(), emailDePrueba, claveDePrueba); err == nil {
		t.Error("la contrasena vieja sigue entrando despues de resembrar")
	}
}

// La invariante se aplica en SQL; aqui solo se comprueba su traduccion. Que la
// sentencia la cumpla de verdad lo prueba repo_integracion_test.go.
//
// Es sobre el superadmin, no sobre el admin: sin superadmin nadie puede volver
// a conceder un permiso, y de ahi no se sale sin abrir la base a mano.
func TestElUltimoSuperadminSaleComo409(t *testing.T) {
	t.Run("al quitarle el rol", func(t *testing.T) {
		s, repo := servicio(t, true)
		repo.errAlQuitarRol = errUltimoSuperadmin

		err := s.QuitarRol(context.Background(), "da39a3ee-5e6b-7000-8000-000000000000", RolSuperadmin)
		if err == nil {
			t.Fatal("se dejo quitar el rol al ultimo superadmin")
		}
		p := problemaDe(t, err)
		if p.Status != http.StatusConflict {
			t.Errorf("estado = %d, se esperaba 409", p.Status)
		}
		if p.Code != httpx.CodeConflict {
			t.Errorf("code = %q, se esperaba %q", p.Code, httpx.CodeConflict)
		}
	})

	t.Run("al deshabilitarlo", func(t *testing.T) {
		s, repo := servicio(t, true)
		repo.errAlDeshabilitar = errUltimoSuperadmin

		_, err := s.Deshabilitar(context.Background(), "da39a3ee-5e6b-7000-8000-000000000000")
		if err == nil {
			t.Fatal("se dejo deshabilitar al ultimo superadmin")
		}
		if p := problemaDe(t, err); p.Status != http.StatusConflict {
			t.Errorf("estado = %d, se esperaba 409", p.Status)
		}
	})
}

func TestQuitarUnRolQueNoTieneEs404(t *testing.T) {
	s, repo := servicio(t, true)
	repo.errAlQuitarRol = errRolNoAsignado

	err := s.QuitarRol(context.Background(), "da39a3ee-5e6b-7000-8000-000000000000", RolStaff)
	if err == nil {
		t.Fatal("no protesto")
	}
	if p := problemaDe(t, err); p.Status != http.StatusNotFound {
		t.Errorf("estado = %d, se esperaba 404", p.Status)
	}
}

func TestElActorLlevaLosPermisosDelUsuario(t *testing.T) {
	s, _ := servicio(t, true)
	u := conUsuario(t, s, emailDePrueba)

	actor, err := s.Actor(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("Actor: %v", err)
	}
	if actor.ID != u.ID {
		t.Errorf("id del actor = %q, se esperaba %q", actor.ID, u.ID)
	}
	if !actor.Can("settings.read") {
		t.Error("el actor no lleva los permisos que devuelve el repositorio")
	}
	if actor.Can("settings.write") {
		t.Error("el actor lleva un permiso que el repositorio no devolvio")
	}
}

// Un hash corrupto en la base no puede confundirse con una contrasena mala: lo
// primero hay que verlo en el log, lo segundo no.
func TestUnHashRotoEnLaBaseNoEsUn401(t *testing.T) {
	s, repo := servicio(t, true)
	conUsuario(t, s, emailDePrueba)
	repo.porEmail[emailDePrueba].hash = "esto-no-es-un-hash"

	_, err := s.Autenticar(context.Background(), emailDePrueba, claveDePrueba)
	if err == nil {
		t.Fatal("se dejo entrar con un hash roto")
	}
	var p *httpx.Problem
	if errors.As(err, &p) {
		t.Fatalf("salio como %d hacia el cliente; un hash corrupto es un fallo del servidor, no del que intenta entrar", p.Status)
	}
	if !errors.Is(err, errHashInvalido) {
		t.Errorf("error = %v; se esperaba el sentinela del hash invalido", err)
	}
}
