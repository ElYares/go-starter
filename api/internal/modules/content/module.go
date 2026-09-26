// Package content es la landing editable: paginas con versiones y bloques
// validados contra un catalogo.
//
// Guardar crea una version; publicar apunta a una. Lo que ve el publico es solo
// la version apuntada, asi que editar no toca lo publicado y revertir es
// publicar una anterior. Ver la Decision 006 y docs/03-modelo-de-datos.md.
package content

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
// servidor de este modulo. Ver docs/05-contratos-api.md.
//go:generate go tool oapi-codegen --config openapi.cfg.yaml ../../../openapi.yaml

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	svc *Service
}

// New devuelve error porque compila el catalogo de bloques. Un esquema mal
// escrito tiene que impedir arrancar, no convertirse en un 500 la primera vez
// que alguien guarda ese tipo de bloque.
func New(pool *pgxpool.Pool) (*Module, error) {
	cat, err := cargarCatalogo(bloquesFS)
	if err != nil {
		return nil, err
	}
	return &Module{svc: &Service{repo: &Repo{pool: pool}, cat: cat}}, nil
}

func (m *Module) Name() string { return "content" }

// area agrupa los permisos de este modulo en la vista de roles.
const area = "Paginas"

// Guardar y publicar son permisos distintos a proposito: quien edita puede no
// poder cambiar lo que ve el publico. Ninguno es sensible: operan el sitio, no
// reparten poder.
func (m *Module) Permissions() []rbac.Permission {
	return []rbac.Permission{
		{Key: "content.page.read", Desc: "Ver las paginas de la landing y sus versiones", Area: area},
		{Key: "content.page.write", Desc: "Crear, guardar y borrar paginas de la landing", Area: area},
		{Key: "content.page.publish", Desc: "Publicar una version de una pagina", Area: area},
	}
}

func (m *Module) Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("content: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

var _ ServerInterface = (*Module)(nil)

// Routes monta los metodos de ServerInterfaceWrapper sobre el router propio,
// ruta por ruta, para conservar el permiso declarado en cada una. Ver
// settings/module.go para el porque completo.
//
// Sobre la seccion 8 del molde —"el usuario A recibe 404 sobre un recurso de
// B"—: NO aplica, y es deliberado. Una pagina es del sitio, no de quien la
// creo; filtrar por `created_by` haria que la portada que sembro la migracion
// no la pudiera editar nadie, y que el dia que se va quien escribio "nosotros",
// la pagina quede huerfana. Lo que decide quien toca una pagina es el permiso.
func (m *Module) Routes(r *httpx.Router) {
	w := &ServerInterfaceWrapper{Handler: m, ErrorHandlerFunc: errorDeParametro}

	r.Group("/api/v1", func(r *httpx.Router) {
		// Lo publico, a la vista y sin guard.
		r.Get("/public/pages/{slug}", w.LeerPaginaPublica)

		r.Group("/pages", func(r *httpx.Router) {
			r.Get("", w.ListarPaginas, rbac.Require("content.page.read"))
			r.Post("", w.CrearPagina, rbac.Require("content.page.write"))
			r.Get("/{id}", w.LeerPagina, rbac.Require("content.page.read"))
			r.Put("/{id}", w.GuardarPagina, rbac.Require("content.page.write"))
			r.Delete("/{id}", w.BorrarPagina, rbac.Require("content.page.write"))
			r.Get("/{id}/versions", w.ListarVersiones, rbac.Require("content.page.read"))
			r.Post("/{id}/publish", w.PublicarPagina, rbac.Require("content.page.publish"))

			// PATCH no existe: guardar escribe una version completa, y una
			// version a medias no es algo que el editor sepa producir.
		})
	})
}

// errorDeParametro sustituye al manejador por omision de lo generado, que
// responde texto plano y sin traceId. El bloque se repite en cada modulo porque
// los tipos de error los genera oapi-codegen dentro del paquete del modulo.
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
