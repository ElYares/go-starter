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
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/paging"
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

type cosa struct {
	ID        string    `json:"id"`
	Nombre    string    `json:"nombre"`
	CreatedAt time.Time `json:"createdAt"`
}

// La lista blanca de este recurso, declarada junto a su repositorio y no en un
// mapa global. Se copia y se ajusta; no se borra.
//
// El desempate que pide NewSpec es obligatorio a proposito: sin el, dos filas
// empatadas en el campo de orden se repiten o se pierden entre paginas, y eso
// se ve como "un registro que desaparecio".
var listado = paging.NewSpec("id").
	SortBy("nombre", "nombre").
	SortBy("createdAt", "created_at").
	FilterBy("nombre", "nombre", paging.Text)

// listar es el molde de coleccion. Lo unico que falta aqui es la consulta.
func (m *Module) listar(w http.ResponseWriter, r *http.Request) {
	p, prob := listado.Parse(r.URL.Query())
	if prob != nil {
		// Un sort o un filtro fuera de la lista blanca sale como 400 antes de
		// tocar la base. Por eso no puede aparecer como un 500 con SQL dentro.
		httpx.WriteProblem(w, r, prob)
		return
	}

	// En el modulo de verdad, aqui va la consulta:
	//
	//	where, args := p.Where(1)
	//	q := fmt.Sprintf("select %s from plantilla_cosas %s %s limit $%d offset $%d",
	//		columnas, where, p.OrderBy(), len(args)+1, len(args)+2)
	//
	// `p.OrderBy()` y `p.Where()` ya vienen resueltos a columnas reales, con el
	// desempate incluido y los valores como parametros.
	httpx.WriteJSON(w, r, http.StatusOK, paging.NewPage([]cosa{}, p, 0))
}

// crear es el molde de alta. La auditoria no aparece por ningun lado a
// proposito: la llena platform/audit dentro del repositorio, leyendo el actor
// del contexto. Ver settings/repo.go para el INSERT completo.
func (m *Module) crear(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, httpx.New(http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"Sin implementar", "Esto es el molde, no un modulo de verdad"))
}
