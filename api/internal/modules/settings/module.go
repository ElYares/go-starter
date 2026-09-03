// Package settings es la configuracion del sitio: marca, navegacion, pie, tema.
//
// Es el primer consumidor del molde a proposito. Podria haber sido un recurso
// de juguete, pero settings ya esta en el alcance base del starter: asi lo que
// prueba el molde es codigo que se queda, no codigo que nace para morir.
package settings

import (
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

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

func (m *Module) Routes(r *httpx.Router) {
	r.Group("/api/v1", func(r *httpx.Router) {
		// Lo publico va bajo su propio prefijo y sin guard, a la vista. Abrir una
		// ruta al mundo tiene que ser una linea que se lee en el diff.
		r.Get("/public/settings", m.listarPublicas)

		// Y lo protegido cierra por omision: sin sesion, 401; con sesion sin el
		// permiso, 403. En la fase 1 nadie tiene sesion todavia, asi que estas
		// rutas responden 401 hasta la fase 2 — y eso es correcto, no un
		// pendiente.
		r.Group("/settings", func(r *httpx.Router) {
			r.Get("", m.listar, rbac.Require("settings.read"))
			r.Post("", m.crear, rbac.Require("settings.write"))
			r.Get("/{key}", m.leer, rbac.Require("settings.read"))
			r.Put("/{key}", m.reemplazar, rbac.Require("settings.write"))

			// DELETE no existe, y es deliberado: el codigo de la landing lee
			// claves por nombre, asi que borrar una no deja un hueco sino una
			// pantalla rota, y sin rastro de que hubo algo ahi. Una clave que
			// sobra se apaga con su valor, no se borra.
			// Ver docs/04-reglas-de-crud.md seccion 8.
		})
	})
}
