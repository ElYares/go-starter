// Package app arranca el servidor y monta lo que cada modulo declara.
//
// Es el unico paquete que conoce el grafo completo: platform no conoce a los
// modulos y un modulo no conoce a otro. Esas dos reglas las verifica
// limites_test.go, no la disciplina.
package app

import (
	"context"
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
	r.Group("/api/v1", func(r *httpx.Router) {
		r.Get("/healthz", a.healthz)
		r.Get("/readyz", a.readyz)

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
