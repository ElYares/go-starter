// Package settings es la configuracion del sitio: marca, navegacion, pie, tema.
//
// Es el primer consumidor del molde a proposito. Podria haber sido un recurso
// de juguete, pero settings ya esta en el alcance base del starter: asi lo que
// prueba el molde es codigo que se queda, no codigo que nace para morir.
package settings

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// El contrato manda: de api/openapi.yaml salen los tipos y la interfaz de
// servidor de este modulo. Se regenera con `go generate ./...` desde api/, y el
// CI falla si lo versionado no coincide. Ver docs/05-contratos-api.md.
//go:generate go tool oapi-codegen --config openapi.cfg.yaml ../../../openapi.yaml

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	svc *Service
}

func New(pool *pgxpool.Pool) *Module {
	return &Module{svc: &Service{repo: &Repo{pool: pool}}}
}

func (m *Module) Name() string { return "settings" }

func (m *Module) Permissions() []rbac.Permission {
	return []rbac.Permission{
		{Key: "settings.read", Desc: "Ver la configuracion del sitio, incluidas las claves privadas"},
		{Key: "settings.write", Desc: "Cambiar la configuracion del sitio"},
	}
}

func (m *Module) Migrations() fs.FS {
	// El error solo puede darse si el //go:embed de arriba no coincide con la
	// carpeta, y eso lo detecta el compilador. Un panic aqui es un bug, no una
	// condicion de ejecucion.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("settings: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

// La afirmacion que convierte el contrato en un error de compilacion.
//
// Si en api/openapi.yaml se renombra una operacion, se le agrega un parametro o
// se le cambia el tipo a uno, esta linea deja de compilar y nombra el metodo
// que ya no cuadra. Es lo que HU-006 pide: que un cambio de contrato rompa la
// compilacion y no la produccion.
var _ ServerInterface = (*Module)(nil)

// Routes es el unico lugar donde este modulo aparece en el router.
//
// Se montan los metodos de ServerInterfaceWrapper —que son http.HandlerFunc
// normales: enlazan los parametros que declara el contrato y despues llaman a
// la interfaz— y NO el registro que trae lo generado (`HandlerWithOptions`).
//
// La razon: aquel monta sobre un mux con middleware global para todas las
// operaciones del modulo. Con eso se perderia el permiso declarado por ruta, y
// con el `Route.Permission` y la comprobacion de arranque que se niega a
// levantar si una ruta exige un permiso que ningun modulo declara (HU-005).
// Montando el wrapper a mano se conservan las dos cosas.
func (m *Module) Routes(r *httpx.Router) {
	w := &ServerInterfaceWrapper{Handler: m, ErrorHandlerFunc: errorDeParametro}

	r.Group("/api/v1", func(r *httpx.Router) {
		// Lo publico va bajo su propio prefijo y sin guard, a la vista. Abrir una
		// ruta al mundo tiene que ser una linea que se lee en el diff.
		r.Get("/public/settings", w.ListarSettingsPublicas)

		// Y lo protegido cierra por omision: sin sesion, 401; con sesion sin el
		// permiso, 403. En la fase 1 nadie tiene sesion todavia, asi que estas
		// rutas responden 401 hasta la fase 2 — y eso es correcto, no un
		// pendiente.
		r.Group("/settings", func(r *httpx.Router) {
			r.Get("", w.ListarSettings, rbac.Require("settings.read"))
			r.Post("", w.CrearSetting, rbac.Require("settings.write"))
			r.Get("/{key}", w.LeerSetting, rbac.Require("settings.read"))
			r.Put("/{key}", w.ReemplazarSetting, rbac.Require("settings.write"))

			// DELETE no existe, y es deliberado: el codigo de la landing lee
			// claves por nombre, asi que borrar una no deja un hueco sino una
			// pantalla rota, y sin rastro de que hubo algo ahi. Una clave que
			// sobra se apaga con su valor, no se borra.
			// Ver docs/04-reglas-de-crud.md seccion 8.
		})
	})
}

// errorDeParametro sustituye al manejador por omision de lo generado, que
// responde con `http.Error`: texto plano y sin traceId.
//
// Sin esto, un `?size=abc` saldria en un formato distinto al de todos los demas
// errores de la API. Es el mismo agujero que el 404 de omision de net/http, y
// se ve igual de bien en el navegador mientras rompe a los clientes.
//
// El bloque se repite en cada modulo porque los tipos de error los genera
// oapi-codegen dentro del paquete del modulo, no en uno compartido.
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
