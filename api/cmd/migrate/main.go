// Comando de migracion. Existe para que migrar sea un paso EXPLICITO del
// despliegue y no un efecto secundario de arrancar el servidor: dos replicas
// aplicando la misma migracion a la vez es una carrera.
//
// En desarrollo no hace falta correrlo a mano: MIGRATE_ON_START lo hace el
// propio servidor. Ver docs/08-infra-local.md.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/elyares/go-starter/api/internal/app"
	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/db"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("configuracion invalida: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := observ.NewLogger(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("no se pudo abrir la conexion", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := app.RunMigrations(ctx, cfg, log, pool); err != nil {
		log.Error("fallo la migracion", "error", err)
		os.Exit(1)
	}

	// El catalogo de permisos va aqui y no en el arranque del servidor: es un
	// cambio de esquema logico, del mismo tipo que una migracion, y por la
	// misma razon tiene que ser un paso explicito. Ver app.SeedPermissions.
	if err := app.SeedPermissions(ctx, cfg, log, pool); err != nil {
		log.Error("fallo la siembra de permisos", "error", err)
		os.Exit(1)
	}

	log.Info("migraciones y permisos al dia")
}
