// Package identity son los usuarios, los roles y el catalogo de permisos.
//
// Es el unico modulo del que otro puede depender —la excepcion escrita en
// limites_test.go— porque todo lo demas necesita saber quien hace la peticion.
// Esa excepcion es el motivo de que aqui no haya nada mas que identidad: cada
// cosa que se meta en este paquete se vuelve importable desde cualquier sitio.
package identity

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// El contrato manda: de api/openapi.yaml salen los tipos y la interfaz de
// servidor de las rutas de sesion de este modulo. Se regenera con
// `go generate ./...` desde api/, y el CI falla si lo versionado no coincide.
// Ver docs/05-contratos-api.md.
//go:generate go tool oapi-codegen --config openapi.cfg.yaml ../../../openapi.yaml

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	svc  *Service
	repo *Repo
	// emisor pone las cookies. Vive en el modulo y no en el service porque
	// escribir cabeceras es cosa de HTTP, y el service no sabe que HTTP existe.
	emisor *auth.Emisor
}

// New recibe si el entorno es de desarrollo, no lo lee del entorno: config es
// el unico paquete que traduce variables, y un modulo que consulta os.Getenv se
// vuelve imposible de probar en los dos modos. Por lo mismo recibe el firmante
// y el emisor ya construidos: la llave de firma y el flag Secure salen de
// config, y este paquete no tiene por que saber de donde.
func New(pool *pgxpool.Pool, dev bool, firmante *auth.Firmante, intentos *auth.Intentos, emisor *auth.Emisor) *Module {
	repo := &Repo{pool: pool}
	return &Module{
		svc:    &Service{repo: repo, dev: dev, firmante: firmante, intentos: intentos},
		repo:   repo,
		emisor: emisor,
	}
}

// Service es lo que app inyecta en los demas modulos cuando necesitan resolver
// al actor de una peticion.
func (m *Module) Service() *Service { return m.svc }

func (m *Module) Name() string { return "identity" }

func (m *Module) Permissions() []rbac.Permission {
	// Los cuatro van marcados como sensibles: son los que reparten poder en vez
	// de usarlo. Quien pueda asignar roles puede darse a si mismo cualquier
	// permiso, asi que concederlos al rol `admin` haria de `admin` un
	// superadmin con otro nombre y la separacion no separaria nada.
	return []rbac.Permission{
		{Key: "identity.user.read", Desc: "Ver las cuentas y sus roles", Sensitive: true},
		{Key: "identity.user.write", Desc: "Crear cuentas, cambiar sus datos y deshabilitarlas", Sensitive: true},
		{Key: "identity.role.read", Desc: "Ver los roles y que permisos concede cada uno", Sensitive: true},
		{Key: "identity.role.assign", Desc: "Asignar y quitar roles a una cuenta", Sensitive: true},
	}
}

func (m *Module) Migrations() fs.FS {
	// El error solo puede darse si el //go:embed de arriba no coincide con la
	// carpeta, y eso lo detecta el compilador. Un panic aqui es un bug, no una
	// condicion de ejecucion.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("identity: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

// SembrarPermisos hace de este modulo el dueno de la tabla `permissions`.
//
// app lo descubre por la interfaz, no por el nombre del paquete: quien tenga
// este metodo recibe el catalogo entero al migrar. Asi un fork que reemplace
// identity por otra cosa sigue funcionando, y uno que la borre del todo
// simplemente no siembra nada.
//
// Los permisos NO los declara este modulo: los declaran todos, y aqui solo se
// guardan. Ver docs/02-modulos.md.
func (m *Module) SembrarPermisos(ctx context.Context, perms []rbac.Permission) error {
	return m.repo.SembrarPermisos(ctx, perms)
}

// La afirmacion que convierte el contrato en un error de compilacion: si en
// api/openapi.yaml se renombra una operacion de sesion o se le cambia un
// parametro, esta linea deja de compilar y nombra el metodo que ya no cuadra.
var _ ServerInterface = (*Module)(nil)

// Routes monta la sesion. El CRUD de cuentas del dashboard sigue sin montarse,
// y eso es deliberado: es de una fase posterior, y un endpoint adelantado seria
// superficie publica sin contrato ni prueba de autorizacion.
//
// Se montan los metodos de ServerInterfaceWrapper y NO `HandlerWithOptions`,
// por la misma razon que en settings: aquel registra sobre un mux con
// middleware global y con eso se perderia el permiso por ruta.
func (m *Module) Routes(r *httpx.Router) {
	w := &ServerInterfaceWrapper{Handler: m, ErrorHandlerFunc: errorDeParametro}

	r.Group("/api/v1/auth", func(r *httpx.Router) {
		// Sin guard, y no es un olvido: quien pide una sesion todavia no tiene
		// ninguna. Lo que si la protege es el CSRF, que corre antes en la
		// cadena global y le exige la cabecera como a cualquier mutacion.
		//
		// Vive fuera de /public a proposito —no devuelve contenido publico,
		// abre una sesion— asi que va escrita como excepcion en la prueba de
		// contrato. Que haya que tocar esa lista es el punto.
		r.Post("/login", w.IniciarSesion)

		// Sesion y nada mas: todo el mundo puede leer su propio perfil, asi que
		// no hay permiso con nombre que pedir. Ver rbac.RequireSession.
		r.Get("/me", w.MiPerfil, rbac.RequireSession())
	})
}

// IniciarSesion responde 204 y cuatro cookies. Sin cuerpo: el perfil se pide
// con `me`, para que haya un solo lugar que defina que sabe el frontend.
func (m *Module) IniciarSesion(w http.ResponseWriter, r *http.Request, _ IniciarSesionParams) {
	// El parametro de la cabecera CSRF se ignora aqui a proposito. Lo declara el
	// contrato para que el cliente generado sepa que hace falta, pero quien la
	// COMPRUEBA es el middleware, antes de llegar hasta aqui. Comprobarla otra
	// vez en el handler seria una segunda implementacion de la misma regla, y
	// la que se olvidaria de actualizar es siempre una de las dos.
	var cuerpo Credenciales
	if p := httpx.DecodeJSON(w, r, &cuerpo); p != nil {
		httpx.WriteProblem(w, r, p)
		return
	}

	sesion, err := m.svc.IniciarSesion(r.Context(), IntentoDeSesion{
		Email:    cuerpo.Email,
		Password: cuerpo.Password,
		IP:       httpx.ClientIP(r),
		// Se recorta porque la columna es `text` y no tiene tope: un
		// User-Agent de un mega entra entero en la base si nadie lo corta, y es
		// una cabecera que escribe el cliente.
		UserAgent: recortar(r.UserAgent(), maxUserAgent),
	})
	if err != nil {
		escribirError(w, r, err)
		return
	}

	m.emisor.Emitir(w, sesion.AccessToken, sesion.RefreshToken, sesion.TokenCSRF)
	httpx.NoContent(w)
}

// MiPerfil devuelve quien tiene esta sesion, con los permisos ya resueltos.
func (m *Module) MiPerfil(w http.ResponseWriter, r *http.Request) {
	// El actor esta garantizado por RequireSession: sin el, el guard respondio
	// 401 y este handler no llego a correr. El `ok` se mira igual porque montar
	// esta ruta sin el guard compilaria, y el sintoma seria un panico por indice
	// vacio en vez de un 401.
	actor, ok := rbac.ActorFrom(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, httpx.Unauthorized())
		return
	}

	perfil, err := m.svc.Perfil(r.Context(), actor.ID)
	if err != nil {
		escribirError(w, r, err)
		return
	}

	// El contrato declara `format: uuid`, asi que el tipo generado pide un UUID
	// y no una cadena. El id sale de una columna `uuid`, de modo que esto no
	// puede fallar; se comprueba igual porque "no puede fallar" y "no compruebo"
	// juntos son como se llega a un panico en produccion.
	id, err := uuid.Parse(perfil.Usuario.ID)
	if err != nil {
		escribirError(w, r, fmt.Errorf("identity: el id %q no es un uuid: %w", perfil.Usuario.ID, err))
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, Perfil{
		Id:          id,
		Email:       perfil.Usuario.Email,
		DisplayName: perfil.Usuario.DisplayName,
		// `[]string{}` y no el slice tal cual: una lista vacia tiene que salir
		// como `[]` y jamas como `null`. Un cliente que hace `.includes()` sobre
		// null revienta, y el contrato promete un array.
		Roles:       oVacio(perfil.Roles),
		Permissions: oVacio(perfil.Permisos),
	})
}

// maxUserAgent es generoso a proposito: los agentes reales rondan los 200
// caracteres y el limite existe para acotar el abuso, no para truncar datos
// legitimos.
const maxUserAgent = 512

func recortar(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func oVacio(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// escribirError manda el *httpx.Problem que venga del service tal cual, y
// cualquier otra cosa como 500.
//
// Que el error del service llegue entero es lo que hace que el 401 del login
// sea IDENTICO en los tres casos —correo que no existe, contrasena mala y
// cuenta deshabilitada— sin que el handler tenga que saber cuantos casos hay.
func escribirError(w http.ResponseWriter, r *http.Request, err error) {
	var p *httpx.Problem
	if errors.As(err, &p) {
		httpx.WriteProblem(w, r, p)
		return
	}
	slog.ErrorContext(r.Context(), "fallo no previsto en identity",
		slog.String("error", err.Error()))
	httpx.WriteProblem(w, r, httpx.Internal())
}

// errorDeParametro traduce los fallos de enlace del codigo generado a la forma
// unica de error. El bloque se repite en cada modulo que genera codigo porque
// oapi-codegen deja estos tipos DENTRO del paquete. Ver settings/module.go.
func errorDeParametro(w http.ResponseWriter, r *http.Request, err error) {
	var (
		requerido  *RequiredParamError
		cabecera   *RequiredHeaderError
		formato    *InvalidParamFormatError
		repetido   *TooManyValuesForParamError
		desempaque *UnmarshalingParamError
	)

	switch {
	case errors.As(err, &requerido):
		httpx.WriteProblem(w, r, httpx.ParamRequired(requerido.ParamName))
	case errors.As(err, &cabecera):
		// Aqui cae el login sin X-XSRF-TOKEN si llegara a pasar el middleware.
		// En la practica no llega: el CSRF corre antes y responde 403.
		httpx.WriteProblem(w, r, httpx.ParamRequired(cabecera.ParamName))
	case errors.As(err, &repetido):
		httpx.WriteProblem(w, r, httpx.ParamRepeated(repetido.ParamName))
	case errors.As(err, &formato):
		httpx.WriteProblem(w, r, httpx.ParamType(formato.ParamName))
	case errors.As(err, &desempaque):
		httpx.WriteProblem(w, r, httpx.ParamType(desempaque.ParamName))
	default:
		httpx.WriteProblem(w, r, httpx.ParamInvalid())
	}
}

// Actor resuelve los permisos efectivos de un usuario. Es lo que el middleware
// de sesion de plataforma necesita para llenar el contexto, y llega hasta el
// por una interfaz que declara `app`, no por un import: plataforma no conoce
// modulos. Ver la Decision 001 y auth.ResolverActor.
//
// La firma habla de `rbac.Actor` —que es de plataforma— y no de tipos de este
// paquete, que es justo lo que permite declararla desde fuera.
func (m *Module) Actor(ctx context.Context, userID string) (rbac.Actor, error) {
	return m.svc.Actor(ctx, userID)
}

// SembrarSuperadminDeDesarrollo es lo que llama `cmd/seed`. La firma habla de
// cadenas y no de tipos de este paquete a proposito: es lo que permite que app
// la declare como interfaz sin importar el modulo.
func (m *Module) SembrarSuperadminDeDesarrollo(ctx context.Context, email, password, nombre string) error {
	_, err := m.svc.SembrarSuperadminDeDesarrollo(ctx, UsuarioNuevo{
		Email:       email,
		Password:    password,
		DisplayName: nombre,
		Roles:       []string{RolSuperadmin},
	})
	return err
}
