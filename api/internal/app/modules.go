package app

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/modules/settings"
	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/db"
)

// modules es la tabla de contenidos del backend, escrita a mano.
//
// No hay descubrimiento automatico ni init() con efectos secundarios: la magia
// funciona hasta que alguien pregunta por que existe una ruta y no hay donde
// mirar.
//
// **El orden importa: es el orden en que corren las migraciones.** Un modulo
// solo puede tener llave foranea hacia otro que se registre antes que el, y en
// la practica eso significa hacia identity. Si dos se necesitan mutuamente, uno
// de los dos esta mal cortado.
func Modules(pool *pgxpool.Pool) []Module {
	return []Module{
		// identity.New(...),   // fase 2: va primero, los demas dependen de el
		settings.New(pool),
		// content.New(...),    // fase 3
		// catalog.New(...),    // <- un fork agrega su dominio aqui
	}
}

// RunMigrations aplica lo pendiente de todos los modulos. La usan dos sitios:
// `cmd/migrate` como paso explicito del despliegue, y el arranque del servidor
// cuando MIGRATE_ON_START esta puesto, que en la practica es solo desarrollo.
func RunMigrations(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	mods := Modules(pool)
	migratables := make([]db.Migratable, 0, len(mods))
	for _, m := range mods {
		migratables = append(migratables, m)
	}
	return db.Migrate(ctx, cfg.DatabaseURL, log, migratables...)
}
