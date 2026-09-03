// Package plantilla es el molde de un modulo. No se registra en modules.go.
//
// Un modulo es un componente autocontenido: trae sus rutas, sus migraciones,
// sus permisos y sus dependencias declaradas. La prueba de que esta bien hecho
// es que borrar su carpeta y su linea del registro deja el proyecto compilando.
package plantilla

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Name() string { return "plantilla" }

func (m *Module) Permissions() []rbac.Permission {
	return []rbac.Permission{
		{Key: "plantilla.read", Desc: "Ver las cosas de este modulo"},
		{Key: "plantilla.write", Desc: "Crear y editar las cosas de este modulo"},
	}
}

func (m *Module) Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("plantilla: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

// Routes es el unico lugar donde este modulo aparece en el router.
func (m *Module) Routes(r *httpx.Router) {
	r.Group("/api/v1", func(r *httpx.Router) {
		r.Get("/cosas", m.listar, rbac.Require("plantilla.read"))
		r.Post("/cosas", m.crear, rbac.Require("plantilla.write"))
	})
}

func (m *Module) listar(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{"content": []any{}})
}

func (m *Module) crear(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, httpx.New(http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"Sin implementar", "Esto es el molde, no un modulo de verdad"))
}
