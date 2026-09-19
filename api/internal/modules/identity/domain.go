package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Los cuatro roles del starter, escritos aqui y sembrados por la migracion. Un
// fork agrega los suyos; estos son los que el codigo nombra.
//
// La diferencia entre los dos primeros no es de grado: `superadmin` reparte
// poder y `admin` opera el sitio. Por eso la invariante de "siempre queda uno"
// es sobre el superadmin —sin el, nadie puede volver a conceder nada— y no
// sobre el admin, que es un rol como cualquier otro.
const (
	RolSuperadmin = "superadmin"
	RolAdmin      = "admin"
	RolStaff      = "staff"
	RolViewer     = "viewer"
)

// Usuario es la forma que sale de este modulo. El hash NO esta aqui a
// proposito: un tipo que lo lleva dentro acaba serializado en una respuesta el
// dia que alguien devuelve el usuario tal cual.
type Usuario struct {
	ID          string
	Email       string
	DisplayName string
	Enabled     bool
	// DevSeed marca al admin que crea `cmd/seed`. Ver Service.Autenticar.
	DevSeed bool
	// MustChangePassword marca una contrasena temporal que asigno otra
	// persona (HU-019). Ver Service.Actor.
	MustChangePassword bool
	Version            int

	CreatedAt time.Time
	UpdatedAt time.Time
	// UpdatedBy es nil cuando la ultima escritura la hizo el sistema —la
	// siembra, una migracion— y no una persona.
	UpdatedBy *string
}

// UsuarioConRoles es la forma que ve el dashboard: la cuenta y las claves de
// sus roles, en una sola lectura.
type UsuarioConRoles struct {
	Usuario
	Roles []string
}

// ModificacionDeUsuario es lo unico que se puede cambiar de una cuenta con un
// guardado. La contrasena, los roles y `enabled` tienen su operacion propia, y
// que este tipo no tenga donde ponerlos es lo que impide que un guardado los
// toque.
type ModificacionDeUsuario struct {
	Email       string
	DisplayName string
}

// SolicitudPendiente es un pedido de contrasena que nadie atendio todavia, con
// lo que hace falta para reconocer la cuenta sin identity.user.read.
type SolicitudPendiente struct {
	ID          string
	UserID      string
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

// RolConPermisos es un rol visto desde la pantalla de roles. Los permisos son
// los de plataforma porque es la forma que ya declaran los modulos.
type RolConPermisos struct {
	Key      string
	Name     string
	Permisos []rbac.Permission
}

// Errores del repositorio. Son sentinelas y no *httpx.Problem a proposito: el
// repositorio no decide codigos HTTP. Quien traduce es el service.
var (
	errNoExiste      = errors.New("identity: el usuario no existe")
	errYaExiste      = errors.New("identity: el correo ya esta registrado")
	errRolNoExiste   = errors.New("identity: el rol no existe")
	errRolNoAsignado = errors.New("identity: el usuario no tiene ese rol")
	errVersion       = errors.New("identity: la version no coincide")

	// errUltimoSuperadmin es la invariante del modulo: siempre queda un
	// superadmin habilitado. Sin ella, quitarse el rol por descuido deja una
	// instalacion en la que nadie puede volver a conceder un permiso ni crear
	// una cuenta, y la unica salida es abrir la base a mano.
	errUltimoSuperadmin = errors.New("identity: es el ultimo superadmin habilitado")

	// errSinAcceso es una sesion vigente de una cuenta que ya no puede entrar:
	// deshabilitada, o la sembrada fuera de desarrollo. No llega al cliente
	// como tal; ver Service.Actor.
	errSinAcceso = errors.New("identity: la cuenta no tiene acceso")

	// errMasPoder: la cuenta a la que se le quiere asignar una contrasena tiene
	// algun permiso que quien la asigna no tiene. Asignarsela seria poder
	// entrar como ella, es decir, darse esos permisos.
	errMasPoder = errors.New("identity: la cuenta tiene permisos que el actor no tiene")
)

// Validacion. Vive aqui y no solo en el contrato porque el contrato declara la
// forma pero no la hace cumplir: el servidor no valida contra el YAML. Ademas
// estas funciones dan el mensaje por campo, que es lo que un `maxLength`
// incumplido no sabe decir.

// largos maximos. El del correo es el del RFC 5321; los otros dos son limites
// de sentido comun que existen para que una peticion enorme falle en la
// validacion y no en la base.
const (
	maxEmail       = 254
	maxDisplayName = 120
	// Doce y no ocho: ocho caracteres se rompen por fuerza bruta con hardware
	// de alquiler. argon2id encarece cada intento, pero no arregla una
	// contrasena que estaba en la primera lista que probaron.
	minPassword = 12
	// El maximo no es una restriccion de seguridad sino de recursos: argon2
	// procesa lo que le den, y un campo de diez megas es una peticion que
	// ocupa un nucleo entero.
	maxPassword = 1024
)

// normalizarEmail solo recorta. NO baja a minusculas a proposito: quien
// garantiza que Ana@casa.com y ana@casa.com son la misma cuenta es el tipo
// `citext` de la columna, y si el codigo normalizara antes, esa garantia
// dejaria de ejercitarse y nadie se enteraria el dia que alguien escriba una
// consulta que se la salta.
func normalizarEmail(email string) string { return strings.TrimSpace(email) }

func validarEmail(email string) *httpx.Problem {
	arroba := strings.IndexByte(email, '@')

	switch {
	case email == "":
		return httpx.BadRequest("El correo es obligatorio", httpx.FieldIssue{
			Field: "email", Code: "required", Message: "Este campo es obligatorio",
		})
	case len(email) > maxEmail:
		return httpx.BadRequest("El correo es demasiado largo", httpx.FieldIssue{
			Field: "email", Code: "max",
			Message: fmt.Sprintf("El maximo es %d caracteres", maxEmail),
		})
	// Una sola arroba, con algo a cada lado y un punto en el dominio. No se
	// valida mas: la expresion "correcta" del RFC 5322 acepta cosas que ningun
	// proveedor entrega y rechaza direcciones que existen. Lo unico que prueba
	// de verdad que un correo es valido es mandarle un mensaje.
	case arroba <= 0 || arroba != strings.LastIndexByte(email, '@'):
		return httpx.BadRequest("El correo no tiene forma de correo", httpx.FieldIssue{
			Field: "email", Code: "format", Message: "Tiene que ser del estilo nombre@dominio.com",
		})
	case !strings.Contains(email[arroba+1:], "."), strings.ContainsAny(email, " \t\n"):
		return httpx.BadRequest("El correo no tiene forma de correo", httpx.FieldIssue{
			Field: "email", Code: "format", Message: "Tiene que ser del estilo nombre@dominio.com",
		})
	}
	return nil
}

func validarPassword(p string) *httpx.Problem {
	switch {
	case p == "":
		return httpx.BadRequest("La contrasena es obligatoria", httpx.FieldIssue{
			Field: "password", Code: "required", Message: "Este campo es obligatorio",
		})
	case len(p) < minPassword:
		return httpx.BadRequest("La contrasena es demasiado corta", httpx.FieldIssue{
			Field: "password", Code: "min",
			Message: fmt.Sprintf("El minimo es %d caracteres", minPassword),
		})
	case len(p) > maxPassword:
		return httpx.BadRequest("La contrasena es demasiado larga", httpx.FieldIssue{
			Field: "password", Code: "max",
			Message: fmt.Sprintf("El maximo es %d caracteres", maxPassword),
		})
	}
	return nil
}

func validarDisplayName(n string) *httpx.Problem {
	n = strings.TrimSpace(n)
	switch {
	case n == "":
		return httpx.BadRequest("El nombre es obligatorio", httpx.FieldIssue{
			Field: "displayName", Code: "required", Message: "Este campo es obligatorio",
		})
	case len(n) > maxDisplayName:
		return httpx.BadRequest("El nombre es demasiado largo", httpx.FieldIssue{
			Field: "displayName", Code: "max",
			Message: fmt.Sprintf("El maximo es %d caracteres", maxDisplayName),
		})
	}
	return nil
}

// IntentoDeSesion es lo que llega del login: las credenciales y el contexto de
// la peticion.
//
// La IP y el agente no son decoracion: la IP es la mitad del limite de intentos
// —la otra es el correo— y las dos quedan en la fila de `refresh_tokens`, que es
// lo unico que despues permite mirar una lista de sesiones abiertas y reconocer
// la que no es tuya.
type IntentoDeSesion struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

// Sesion es lo que sale de iniciar sesion: los tres valores que van a cookies,
// mas quien resulto ser.
//
// El service los emite y el handler solo los coloca. Es a proposito: asi la
// regla de que un login emite EXACTAMENTE estas tres cosas se prueba sin HTTP,
// y un handler no puede olvidarse de una.
type Sesion struct {
	Usuario      Usuario
	Roles        []string
	AccessToken  string
	RefreshToken string
	TokenCSRF    string
}

// SesionNueva es la fila de `refresh_tokens`. Lleva el HASH, nunca el token.
//
// Que el campo se llame TokenHash y sea []byte es la defensa: un `Token string`
// aqui haria que guardar el valor en claro fuera un descuido de una linea en
// vez de un cambio de tipo que no compila.
type SesionNueva struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiraEn  time.Time
	UserAgent string
	IP        string
}

// PerfilDeUsuario es lo que devuelve `GET /auth/me`: quien eres y que puedes hacer.
//
// Los permisos van resueltos, no los roles a secas, para que el frontend pueda
// ocultar lo que no aplica sin reimplementar el modelo de permisos. Ocultar es
// conveniencia; la autorizacion de verdad vive en el guard de cada ruta.
// Se llama asi y no `Perfil` porque ese nombre ya lo ocupa el tipo que genera
// el contrato, que es la forma de CABLE. Son dos cosas distintas a proposito:
// esta lleva el `Usuario` del dominio y aquella los campos que viajan en JSON.
type PerfilDeUsuario struct {
	Usuario  Usuario
	Roles    []string
	Permisos []string
}

// RenovacionDeSesion es lo que llega al refresh: el `rt` de la cookie y el
// contexto de la peticion, que queda en la fila nueva igual que en el login.
type RenovacionDeSesion struct {
	RefreshToken string
	IP           string
	UserAgent    string
}

// RefreshGuardado es una fila de `refresh_tokens` vista desde el refresh.
//
// `ReemplazadoPor` apunta al SUCESOR, no al predecesor: "a esta fila la
// reemplazo aquella". Asi "ya se roto?" se responde con la fila que ya se tiene
// en la mano, sin preguntarle a la base quien apunta a mi. Es la Decision 012
// del proyecto hermano, que llego a esa forma despues de haber hecho la otra.
type RefreshGuardado struct {
	ID             string
	UserID         string
	ReemplazadoPor *string
	RevocadoEn     *time.Time
	ExpiraEn       time.Time
}
