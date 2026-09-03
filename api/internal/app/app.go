// Package app arranca el servidor y monta lo que cada modulo declara.
//
// Es el unico paquete que conoce el grafo completo: platform no conoce a los
// modulos y un modulo no conoce a otro. Esas dos reglas las verifica
// limites_test.go, no la disciplina.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/db"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// El contrato manda: de api/openapi.yaml salen los tipos y la interfaz de
// servidor de las rutas de plataforma. Se regenera con `go generate ./...`
// desde api/. Ver docs/05-contratos-api.md.
//go:generate go tool oapi-codegen --config openapi.cfg.yaml ../../openapi.yaml

// Si en el contrato se renombra `healthz` o se le agrega un parametro, esta
// linea deja de compilar. Ver internal/modules/settings/module.go.
var _ ServerInterface = (*App)(nil)

type App struct {
	cfg    config.Config
	log    *slog.Logger
	pool   *pgxpool.Pool
	spec   []byte
	perms  *rbac.Registry
	router *httpx.Router
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	a := &App{cfg: cfg, log: log, pool: pool}

	if cfg.APIDocsEnabled {
		// El contrato se lee una vez al arrancar. Si no esta donde se espera, la
		// UI se apaga y se avisa: mejor eso que un 500 al abrir /docs.
		spec, err := os.ReadFile("openapi.yaml")
		if err != nil {
			log.Warn("no se pudo leer openapi.yaml; la UI de la API queda apagada",
				slog.String("error", err.Error()))
		} else {
			a.spec = spec
		}
	}

	if cfg.MigrateOnStart {
		if err := RunMigrations(ctx, cfg, log, pool); err != nil {
			return nil, fmt.Errorf("migrando: %w", err)
		}
	}

	if err := a.montar(Modules(pool)); err != nil {
		return nil, err
	}

	return a, nil
}

// montar cataloga los permisos declarados, deja que cada modulo registre sus
// rutas y despues revisa que ninguna exija un permiso que nadie declaro.
//
// Ese ultimo paso es el que justifica que el router guarde las rutas: sin el,
// una clave mal escrita se veria en produccion como un 403 permanente e
// inexplicable, porque el guard rechazaria a todos, incluido el admin.
func (a *App) montar(mods []Module) error {
	var perms []rbac.Permission
	for _, m := range mods {
		perms = append(perms, m.Permissions()...)
	}

	reg, err := rbac.NewRegistry(perms)
	if err != nil {
		return err
	}
	a.perms = reg

	r := httpx.NewRouter()
	a.rutasDePlataforma(r)

	for _, m := range mods {
		m.Routes(r)
		a.log.Debug("modulo montado", slog.String("modulo", m.Name()))
	}

	if err := rbac.VerifyRoutes(r.Routes(), reg); err != nil {
		return err
	}

	a.router = r
	return nil
}

// rutasDePlataforma son las que no pertenecen a ningun modulo: salud y, en
// desarrollo, la UI del contrato.
func (a *App) rutasDePlataforma(r *httpx.Router) {
	w := &ServerInterfaceWrapper{Handler: a, ErrorHandlerFunc: errorDeParametro}

	r.Group("/api/v1", func(r *httpx.Router) {
		r.Get("/healthz", w.Healthz)
		r.Get("/readyz", w.Readyz)

		// Estas dos NO estan en el contrato, y es deliberado: son la UI de
		// exploracion, viven solo con API_DOCS_ENABLED y no son superficie
		// publica de la API. contrato_test.go las conoce y las exime.
		if a.spec != nil {
			r.Get("/openapi.yaml", a.openapiSpec)
			r.Get("/docs", a.apiDocs)
		}
	})
}

func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

// Handler arma la cadena global de middleware.
//
// El orden es contrato: traceId primero porque todo lo demas lo registra, y
// recover antes del logger para que un panico tambien quede registrado.
// Ver docs/06-flujos.md seccion 6.
func (a *App) Handler() http.Handler {
	return observ.Chain(a.router.Handler(),
		observ.TraceID,
		httpx.Recover(a.log),
		observ.RequestLogger(a.log),
	)
}

// errorDeParametro traduce los fallos de enlace del codigo generado a la forma
// unica de error. Sin el, lo generado responde con `http.Error`: texto plano y
// sin traceId, que es el mismo agujero que el 404 de omision de net/http.
//
// El `switch` se repite en cada paquete que genera codigo porque oapi-codegen
// deja estos tipos de error dentro del paquete, no en uno compartido. Lo que si
// se comparte es el texto de la respuesta, que vive en httpx.
//
// Hoy ninguna ruta de plataforma declara parametros, asi que esto no se dispara.
// Existe para que el dia que alguna los declare no aparezca un texto plano en
// medio de una API que promete problem+json.
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
