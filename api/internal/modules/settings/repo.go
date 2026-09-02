package settings

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

// listar trae la configuracion. `soloPublicas` se resuelve EN LA CONSULTA, no
// filtrando en Go despues de traer todo: una clave privada que no sale de la
// base no se puede filtrar mal por descuido mas arriba.
func (r *Repo) listar(ctx context.Context, soloPublicas bool) ([]Setting, error) {
	const q = `
		select key, value, is_public, version, updated_at
		  from settings
		 where (not $1::boolean) or is_public
		 order by key asc`

	rows, err := r.pool.Query(ctx, q, soloPublicas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Setting, error) {
		var s Setting
		err := row.Scan(&s.Key, &s.Value, &s.IsPublic, &s.Version, &s.UpdatedAt)
		return s, err
	})
}
