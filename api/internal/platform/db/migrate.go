package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // driver database/sql para goose
	"github.com/pressly/goose/v3"
)

// Migratable es lo que el runner necesita de un modulo. Se declara aqui, del
// lado del consumidor, para que platform no tenga que conocer el tipo Module de
// app: eso seria un import hacia arriba y romperia la regla 1.
type Migratable interface {
	Name() string
	Migrations() fs.FS
}

// Migrate aplica lo pendiente de cada modulo, EN EL ORDEN EN QUE LLEGAN.
//
// Cada modulo lleva su propia tabla de versiones, `schema_migrations_<modulo>`.
// Con una tabla compartida, dos ramas que agregan una migracion en modulos
// distintos conflictuan sin razon, y el orden de dos modulos independientes se
// vuelve global y fragil.
//
// Corre como paso explicito, no al arrancar el servidor: dos replicas migrando
// a la vez es una carrera. Ver docs/08-infra-local.md.
func Migrate(ctx context.Context, url string, log *slog.Logger, modules ...Migratable) error {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("abriendo conexion para migrar: %w", err)
	}
	defer conn.Close()

	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("la base no responde: %w", err)
	}

	for _, m := range modules {
		fsys := m.Migrations()
		if fsys == nil {
			continue
		}

		provider, err := goose.NewProvider(
			goose.DialectPostgres, conn, fsys,
			goose.WithTableName("schema_migrations_"+m.Name()),
		)
		if err != nil {
			return fmt.Errorf("modulo %s: %w", m.Name(), err)
		}

		aplicadas, err := provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("modulo %s: %w", m.Name(), err)
		}

		for _, r := range aplicadas {
			log.Info("migracion aplicada",
				slog.String("modulo", m.Name()),
				slog.String("archivo", r.Source.Path),
				slog.Int64("version", r.Source.Version))
		}
	}

	return nil
}
