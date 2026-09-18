package media

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/ids"
)

// registro es la fila: el Medio del contrato mas la llave del almacenamiento,
// que no sale nunca por la API.
type registro struct {
	Medio
	llave string
}

// nuevo es lo que el service ya comprobo y quiere guardar.
type nuevo struct {
	sha256 []byte
	mime   string
	tamano int64
	ancho  int
	alto   int
	nombre *string
	llave  string
}

type Repo struct {
	pool *pgxpool.Pool
}

const columnas = `id, sha256, mime, size_bytes, width, height, original_name, storage_key, created_at, created_by`

func (r *Repo) obtener(ctx context.Context, id uuid.UUID) (registro, error) {
	return uno(r.pool.QueryRow(ctx, `select `+columnas+` from media where id = $1`, id))
}

func (r *Repo) porHash(ctx context.Context, sum []byte) (registro, error) {
	return uno(r.pool.QueryRow(ctx, `select `+columnas+` from media where sha256 = $1`, sum))
}

// insertar devuelve la fila y si la creo. Si otra subida con los mismos bytes
// gano la carrera, devuelve la suya y `false`: lo decide el indice unico en la
// misma sentencia, no un select previo, que dejaria una ventana entre mirar y
// escribir en la que las dos subidas creen ser la primera.
func (r *Repo) insertar(ctx context.Context, n nuevo) (registro, bool, error) {
	id, err := ids.New()
	if err != nil {
		return registro{}, false, err
	}
	sello, err := audit.ForAppend(ctx)
	if err != nil {
		return registro{}, false, err
	}

	q := fmt.Sprintf(`insert into media (id, sha256, mime, size_bytes, width, height, original_name, storage_key, %s)
		values ($1, $2, $3, $4, $5, $6, $7, $8, %s)
		on conflict (sha256) do nothing
		returning `+columnas, sello.ColumnList(), sello.Placeholders(9))
	args := append([]any{id, n.sha256, n.mime, n.tamano, n.ancho, n.alto, n.nombre, n.llave}, sello.Values...)

	creado, err := uno(r.pool.QueryRow(ctx, q, args...))
	if errors.Is(err, errNoExiste) {
		// `do nothing` no devuelve fila: ya estaba.
		existente, err := r.porHash(ctx, n.sha256)
		return existente, false, err
	}
	return creado, err == nil, err
}

func uno(row pgx.Row) (registro, error) {
	var (
		m   registro
		sum []byte
		by  pgtype.UUID
	)
	err := row.Scan(&m.Id, &sum, &m.Mime, &m.SizeBytes, &m.Width, &m.Height, &m.OriginalName, &m.llave, &m.CreatedAt, &by)
	if errors.Is(err, pgx.ErrNoRows) {
		return registro{}, errNoExiste
	}
	if err != nil {
		return registro{}, err
	}
	m.Sha256 = hex.EncodeToString(sum)
	m.Url = urlPublica(m.Id)
	if by.Valid {
		u := openapi_types.UUID(by.Bytes)
		m.CreatedBy = &u
	}
	return m, nil
}
