// Package app arranca el servidor y monta lo que cada modulo declara.
//
// En la fase 0 no hay modulos todavia: solo salud y, en desarrollo, la UI de
// exploracion del contrato. El registro explicito de modulos llega en la
// fase 1 (docs/02-modulos.md).
package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

type App struct {
	cfg  config.Config
	log  *slog.Logger
	pool *pgxpool.Pool
	spec []byte
}

// New abre el pool sin conectar: pgxpool conecta de forma perezosa, asi que el
// servidor levanta aunque la base este caida y /readyz lo reporta. Es lo que se
// quiere: un proceso vivo que dice claramente que no esta listo es mas util que
// uno que no arranca.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
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

	return a, nil
}

func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

// Handler arma el arbol de rutas y la cadena de middleware.
//
// El orden de la cadena es contrato: traceId primero porque todo lo demas lo
// registra, y recover antes del logger para que un panico tambien quede
// registrado. Ver docs/06-flujos.md seccion 6.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", a.healthz)
	mux.HandleFunc("GET /api/v1/readyz", a.readyz)

	if a.spec != nil {
		mux.HandleFunc("GET /api/v1/openapi.yaml", a.openapiSpec)
		mux.HandleFunc("GET /api/v1/docs", a.apiDocs)
	}

	return observ.Chain(mux,
		observ.TraceID,
		observ.Recover(a.log),
		observ.RequestLogger(a.log),
	)
}
