// Package media guarda las imagenes del sitio: el logo, las del contenido.
//
// Un archivo es su contenido: se identifica por el SHA-256 de sus bytes, asi
// que subir dos veces la misma imagen devuelve el registro que ya existia. El
// tipo sale de los bytes, nunca del nombre. Ver docs/06-flujos.md seccion 5.
package media

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
	"github.com/elyares/go-starter/api/internal/platform/storage"
)

// El contrato manda: de api/openapi.yaml salen los tipos y la interfaz de
// servidor de este modulo. Ver docs/05-contratos-api.md.
//go:generate go tool oapi-codegen --config openapi.cfg.yaml ../../../openapi.yaml

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	svc *Service
}

func New(pool *pgxpool.Pool, store storage.Store) *Module {
	return &Module{svc: &Service{repo: &Repo{pool: pool}, store: store}}
}

func (m *Module) Name() string { return "media" }

// Ninguno es sensible: subir una imagen opera el sitio, no reparte poder. Con
// la Decision 023, el admin de una instalacion que ya existia los recibe en el
// siguiente despliegue.
func (m *Module) Permissions() []rbac.Permission {
	return []rbac.Permission{
		{Key: "media.read", Desc: "Ver los datos de las imagenes subidas"},
		{Key: "media.upload", Desc: "Subir imagenes"},
	}
}

func (m *Module) Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("media: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

var _ ServerInterface = (*Module)(nil)

// Routes monta los metodos de ServerInterfaceWrapper sobre el router propio,
// ruta por ruta, para conservar el permiso declarado en cada una. Ver
// settings/module.go para el porque completo.
//
// Del molde de docs/04-reglas-de-crud.md faltan cuatro rutas, y es deliberado:
//   - listar y borrar llegan con la biblioteca de medios. Borrar exige saber
//     quien usa cada archivo, y eso todavia no existe
//   - PUT y PATCH no tienen sentido: el archivo ES su contenido, y cambiarlo
//     es subir otro
//
// Tampoco aplica "el usuario A recibe 404 sobre un recurso de B": una imagen es
// del sitio, igual que una pagina (Decision 021).
func (m *Module) Routes(r *httpx.Router) {
	w := &ServerInterfaceWrapper{Handler: m, ErrorHandlerFunc: errorDeParametro}

	r.Group("/api/v1", func(r *httpx.Router) {
		// Lo publico, a la vista y sin guard: es lo que pide un <img>.
		r.Get("/public/media/{id}", w.LeerMedioPublico)

		r.Group("/media", func(r *httpx.Router) {
			r.Post("", w.SubirMedio, rbac.Require("media.upload"))
			r.Get("/{id}", w.LeerMedio, rbac.Require("media.read"))
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
