// Package db abre la conexion y aplica las migraciones de cada modulo.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open no conecta: pgxpool lo hace de forma perezosa. Es deliberado. El
// servidor levanta aunque la base este caida y /readyz lo reporta, que es mas
// util que un proceso que no arranca y no puede explicar por que.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL invalida: %w", err)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}
