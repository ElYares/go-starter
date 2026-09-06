package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/ids"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// repositorio es la costura entre el service y Postgres. Existe para que las
// reglas —la invariante del ultimo admin, el rechazo del admin de desarrollo
// fuera de dev— se prueben sin una base, que es la mitad de lo que hay en CI.
type repositorio interface {
	crear(ctx context.Context, u Usuario, hash string) (Usuario, error)
	porID(ctx context.Context, id string) (Usuario, error)
	paraAutenticar(ctx context.Context, email string) (Usuario, string, error)
	actualizarCredencial(ctx context.Context, email, hash, displayName string) (Usuario, error)
	permisosDe(ctx context.Context, userID string) ([]string, error)
	rolesDe(ctx context.Context, userID string) ([]string, error)
	asignarRol(ctx context.Context, userID, rol string) error
	quitarRol(ctx context.Context, userID, rol string) error
	deshabilitar(ctx context.Context, userID string) (Usuario, error)
}

type Service struct {
	repo repositorio
	// dev decide si las credenciales sembradas valen. No se lee del entorno
	// aqui: llega desde config, que es el unico sitio que traduce el entorno.
	dev bool
}

// UsuarioNuevo es lo que hace falta para dar de alta a alguien. La contrasena
// entra en claro y sale hasheada sin pasar por ningun otro sitio: no hay un
// tipo intermedio que pueda acabar en un log.
type UsuarioNuevo struct {
	Email       string
	Password    string
	DisplayName string
	Roles       []string
}

// Crear da de alta a un usuario y le asigna sus roles.
//
// No es un endpoint todavia: el CRUD de usuarios desde el dashboard llega en
// una fase posterior. Hoy lo usa el seed, y es lo que esa fase va a montar.
func (s *Service) Crear(ctx context.Context, n UsuarioNuevo) (Usuario, error) {
	return s.crear(ctx, n, false)
}

func (s *Service) crear(ctx context.Context, n UsuarioNuevo, devSeed bool) (Usuario, error) {
	email := normalizarEmail(n.Email)
	if prob := validarEmail(email); prob != nil {
		return Usuario{}, prob
	}
	if prob := validarPassword(n.Password); prob != nil {
		return Usuario{}, prob
	}
	if prob := validarDisplayName(n.DisplayName); prob != nil {
		return Usuario{}, prob
	}

	hash, err := Hash(n.Password)
	if err != nil {
		return Usuario{}, err
	}

	id, err := ids.NewString()
	if err != nil {
		return Usuario{}, err
	}

	creado, err := s.repo.crear(ctx, Usuario{
		ID:          id,
		Email:       email,
		DisplayName: strings.TrimSpace(n.DisplayName),
		Enabled:     true,
		DevSeed:     devSeed,
	}, hash)
	if err != nil {
		return Usuario{}, traducirEstado(err)
	}

	for _, rol := range n.Roles {
		if err := s.repo.asignarRol(ctx, creado.ID, rol); err != nil {
			return Usuario{}, traducirEstado(err)
		}
	}
	return creado, nil
}

// Autenticar comprueba la contrasena. Es la mitad de CU-001 que se puede
// escribir sin cookies: el endpoint, la sesion y el limite de intentos son de
// aquella historia; esto es lo que necesita para tener a quien dejar entrar.
//
// El hash se verifica AUNQUE el usuario no exista o este deshabilitado. Salir
// antes haria que un correo desconocido respondiera en microsegundos y uno
// real en decenas de milisegundos, y esa diferencia es medible por la red:
// convierte el login en un oraculo de que correos estan registrados.
func (s *Service) Autenticar(ctx context.Context, email, password string) (Usuario, error) {
	u, hash, err := s.repo.paraAutenticar(ctx, normalizarEmail(email))
	if err != nil && !errors.Is(err, errNoExiste) {
		return Usuario{}, err
	}

	if hash == "" {
		// No hay usuario: se gasta el mismo trabajo contra un hash de descarte
		// para que el tiempo de respuesta no delate nada.
		hash = hashDeDescarte()
	}

	coincide, errHash := Verify(password, hash)
	if errHash != nil {
		// Un hash corrupto en la base no es "contrasena mala": hay que verlo en
		// el log, no dejarlo pasar como un intento fallido mas.
		return Usuario{}, fmt.Errorf("identity: el usuario %q tiene un hash ilegible: %w", u.ID, errHash)
	}

	switch {
	case err != nil, !coincide, !u.Enabled:
		return Usuario{}, httpx.Unauthorized()

	// La cuenta que siembra `cmd/seed` solo entra en desarrollo. La marca viaja
	// en la fila, asi que sigue valiendo si esa base acaba copiada a otro
	// entorno: es ahi, y no en la maquina del que sembro, donde importa.
	case u.DevSeed && !s.dev:
		return Usuario{}, httpx.Unauthorized()
	}

	return u, nil
}

// hashDeDescarte es un argon2id de una contrasena que nadie tiene. Existe solo
// para gastar el mismo tiempo cuando el correo no existe. Ver Autenticar.
//
// Se calcula una vez y de verdad, no se escribe a mano: un literal inventado no
// es un hash valido, Verify lo rechazaria por formato y el caso "no existe"
// saldria por la rama del error en vez de por la del tiempo constante. Es decir,
// el arreglo dejaria de arreglar y nadie lo notaria.
var hashDeDescarte = sync.OnceValue(func() string {
	h, err := Hash("no es la contrasena de nadie")
	if err != nil {
		// Sin aleatoriedad no hay hash que valga, ni para este ni para el login.
		panic("identity: no se pudo calcular el hash de descarte: " + err.Error())
	}
	return h
})

// Actor arma el rbac.Actor de un usuario: sus permisos efectivos, por sus
// roles. Es lo que el middleware de sesion de CU-001 va a poner en el contexto.
func (s *Service) Actor(ctx context.Context, userID string) (rbac.Actor, error) {
	permisos, err := s.repo.permisosDe(ctx, userID)
	if err != nil {
		return rbac.Actor{}, err
	}
	return rbac.Actor{ID: userID, Permissions: permisos}, nil
}

func (s *Service) Roles(ctx context.Context, userID string) ([]string, error) {
	roles, err := s.repo.rolesDe(ctx, userID)
	return roles, traducirEstado(err)
}

func (s *Service) AsignarRol(ctx context.Context, userID, rol string) error {
	return traducirEstado(s.repo.asignarRol(ctx, userID, rol))
}

// QuitarRol y Deshabilitar comparten la invariante: siempre queda un superadmin
// habilitado. La comprueba el repositorio en la misma sentencia que escribe;
// aqui solo se traduce el resultado.
func (s *Service) QuitarRol(ctx context.Context, userID, rol string) error {
	return traducirEstado(s.repo.quitarRol(ctx, userID, rol))
}

func (s *Service) Deshabilitar(ctx context.Context, userID string) (Usuario, error) {
	u, err := s.repo.deshabilitar(ctx, userID)
	return u, traducirEstado(err)
}

// SembrarSuperadminDeDesarrollo es lo que corre `cmd/seed`. Se puede correr dos
// veces: la segunda reescribe la contrasena en vez de fallar por el correo
// repetido, que es lo que hace util un seed durante el desarrollo.
func (s *Service) SembrarSuperadminDeDesarrollo(ctx context.Context, n UsuarioNuevo) (Usuario, error) {
	if !s.dev {
		// Cinturon, ademas del de cmd/seed. Este es el que sigue puesto si
		// manana alguien llama a esta funcion desde otro sitio.
		return Usuario{}, errors.New(
			"identity: el superadmin de desarrollo solo se siembra con APP_ENV=dev")
	}

	creado, err := s.crear(ctx, n, true)
	if err == nil {
		return creado, nil
	}

	var prob *httpx.Problem
	if !errors.As(err, &prob) || prob.Status != 409 {
		return Usuario{}, err
	}

	// Ya existia: se reescribe la credencial conservando el id.
	email := normalizarEmail(n.Email)
	hash, err := Hash(n.Password)
	if err != nil {
		return Usuario{}, err
	}

	u, err := s.repo.actualizarCredencial(ctx, email, hash, strings.TrimSpace(n.DisplayName))
	if err != nil {
		return Usuario{}, traducirEstado(err)
	}

	for _, rol := range n.Roles {
		if err := s.repo.asignarRol(ctx, u.ID, rol); err != nil {
			return Usuario{}, traducirEstado(err)
		}
	}
	return u, nil
}

// traducirEstado es el unico lugar donde un sentinela del repositorio se
// convierte en un codigo HTTP. Concentrarlo aqui evita que dos operaciones
// contesten distinto a la misma situacion.
func traducirEstado(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNoExiste), errors.Is(err, errRolNoAsignado):
		return httpx.NotFound()
	case errors.Is(err, errYaExiste):
		return httpx.Conflict("Ya hay una cuenta con ese correo")
	case errors.Is(err, errRolNoExiste):
		return httpx.Conflict("Ese rol no existe")
	case errors.Is(err, errUltimoSuperadmin):
		return httpx.Conflict(
			"Es el ultimo superadmin habilitado. Dale el rol a alguien mas antes de quitarselo a este")
	default:
		return err
	}
}
