package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/ids"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// Las listas blancas, junto al repositorio. Las columnas son las de las tablas
// derivadas de abajo (`paginas`, `versiones`), no las de las tablas reales:
// asi el titulo del borrador se puede ordenar sin que paging sepa de joins.
var (
	listadoDePaginas = paging.NewSpec("id").
				SortBy("slug", "slug").
				SortBy("updatedAt", "updated_at").
				FilterBy("published", "published", paging.Bool).
				DefaultOrder(paging.Order{Column: "updated_at", Desc: true})

	// El numero es unico dentro de la pagina: sirve de desempate por si solo.
	listadoDeVersiones = paging.NewSpec("number").
				SortBy("number", "number").
				DefaultOrder(paging.Order{Column: "number", Desc: true})
)

var (
	errNoExiste     = errors.New("content: la pagina no existe")
	errSlugOcupado  = errors.New("content: el slug ya lo usa otra pagina")
	errVersion      = errors.New("content: la version no coincide")
	errVersionAjena = errors.New("content: la version no es de esta pagina")
)

// borradorDe es la ultima version de cada pagina. Es un LATERAL y no un
// `max(number)` agrupado porque hace falta la fila entera, no solo el numero.
const borradorDe = `join lateral (
		select * from page_versions v
		 where v.page_id = p.id
		 order by v.number desc
		 limit 1
	) b on true
	left join page_versions pub on pub.id = p.published_version_id`

// fuenteDePaginas es la tabla derivada sobre la que paging aplica filtro, orden
// y limite.
const fuenteDePaginas = `(
	select p.id, p.slug, b.title, p.version, b.number as draft_number,
	       pub.number as published_number,
	       p.published_version_id is not null as published,
	       p.updated_at, p.updated_by
	  from pages p ` + borradorDe + `
) paginas`

type Repo struct {
	pool *pgxpool.Pool
}

func (r *Repo) pagina(ctx context.Context, p paging.Params) ([]PaginaResumen, int64, error) {
	where, args := p.Where(1)

	var total int64
	if err := r.pool.QueryRow(ctx, `select count(*) from `+fuenteDePaginas+` `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	q := fmt.Sprintf(`select id, slug, title, version, draft_number, published_number, updated_at, updated_by
		  from %s %s %s limit $%d offset $%d`,
		fuenteDePaginas, where, p.OrderBy(), len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (PaginaResumen, error) {
		var (
			s  PaginaResumen
			by pgtype.UUID
		)
		err := row.Scan(&s.Id, &s.Slug, &s.Title, &s.Version, &s.DraftNumber, &s.PublishedNumber, &s.UpdatedAt, &by)
		s.UpdatedBy = uuidONulo(by)
		return s, err
	})
	return items, total, err
}

// versiones distingue "la pagina no existe" de "no tiene versiones" en la misma
// consulta del conteo. Por construccion una pagina siempre tiene al menos una,
// pero un 200 con la lista vacia para un id que no existe mentiria.
func (r *Repo) versiones(ctx context.Context, id uuid.UUID, p paging.Params) ([]VersionResumen, int64, error) {
	var (
		total  int64
		existe bool
	)
	err := r.pool.QueryRow(ctx,
		`select (select count(*) from page_versions where page_id = $1),
		        exists (select 1 from pages where id = $1)`, id).Scan(&total, &existe)
	if err != nil {
		return nil, 0, err
	}
	if !existe {
		return nil, 0, errNoExiste
	}

	where, args := p.Where(2)
	q := fmt.Sprintf(`select id, number, title, note, published, created_at, created_by
		  from (
			select v.id, v.number, v.title, v.note, v.created_at, v.created_by,
			       coalesce(v.id = p.published_version_id, false) as published
			  from page_versions v
			  join pages p on p.id = v.page_id
			 where v.page_id = $1
		  ) versiones %s %s limit $%d offset $%d`,
		where, p.OrderBy(), len(args)+2, len(args)+3)

	rows, err := r.pool.Query(ctx, q, append(append([]any{id}, args...), p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (VersionResumen, error) {
		var (
			v  VersionResumen
			by pgtype.UUID
		)
		err := row.Scan(&v.Id, &v.Number, &v.Title, &v.Note, &v.Published, &v.CreatedAt, &by)
		v.CreatedBy = uuidONulo(by)
		return v, err
	})
	return items, total, err
}

func (r *Repo) obtener(ctx context.Context, id uuid.UUID) (Pagina, error) {
	return obtenerCon(ctx, r.pool, id)
}

// consultor es lo comun a la pool y a una transaccion. Leer con la
// transaccion abierta es lo que hace que la respuesta de un guardado sea la
// fila que ese guardado escribio, y no la de otro que entro despues.
type consultor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func obtenerCon(ctx context.Context, db consultor, id uuid.UUID) (Pagina, error) {
	const q = `select p.id, p.slug, p.version, p.created_at, p.updated_at, p.updated_by,
	                  b.id, b.number, b.title, b.seo_title, b.seo_description, b.blocks, b.note,
	                  p.published_version_id, pub.number
	             from pages p ` + borradorDe + `
	            where p.id = $1`

	rows, err := db.Query(ctx, q, id)
	if err != nil {
		return Pagina{}, err
	}
	defer rows.Close()

	pagina, err := pgx.CollectExactlyOneRow(rows, func(row pgx.CollectableRow) (Pagina, error) {
		var (
			p       Pagina
			by      pgtype.UUID
			pubID   pgtype.UUID
			bloques []byte
		)
		err := row.Scan(&p.Id, &p.Slug, &p.Version, &p.CreatedAt, &p.UpdatedAt, &by,
			&p.DraftVersionId, &p.DraftNumber, &p.Title, &p.SeoTitle, &p.SeoDescription, &bloques, &p.Note,
			&pubID, &p.PublishedNumber)
		if err != nil {
			return Pagina{}, err
		}
		p.UpdatedBy = uuidONulo(by)
		p.PublishedVersionId = uuidONulo(pubID)
		p.Blocks, err = leerBloques(bloques)
		return p, err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Pagina{}, errNoExiste
	}
	return pagina, err
}

// publica lee la version APUNTADA, nunca la ultima. El join por
// published_version_id es lo que hace que una pagina nunca publicada no salga:
// con el puntero nulo no hay fila.
func (r *Repo) publica(ctx context.Context, slug string) (PaginaPublica, error) {
	const q = `select p.slug, v.title, v.seo_title, v.seo_description, v.blocks
	             from pages p
	             join page_versions v on v.id = p.published_version_id
	            where p.slug = $1`

	var (
		p       PaginaPublica
		bloques []byte
	)
	err := r.pool.QueryRow(ctx, q, slug).Scan(&p.Slug, &p.Title, &p.SeoTitle, &p.SeoDescription, &bloques)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaginaPublica{}, errNoExiste
	}
	if err != nil {
		return PaginaPublica{}, err
	}
	p.Blocks, err = leerBloques(bloques)
	return p, err
}

// crear escribe la pagina y su version 1 en una transaccion. Sin ella, un fallo
// entre las dos sentencias deja una pagina sin versiones, que ninguna lectura
// sabe mostrar.
func (r *Repo) crear(ctx context.Context, b borrador) (Pagina, error) {
	id, err := ids.New()
	if err != nil {
		return Pagina{}, err
	}
	sello, err := audit.ForInsert(ctx)
	if err != nil {
		return Pagina{}, err
	}

	var creada Pagina
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := fmt.Sprintf(`insert into pages (id, slug, %s) values ($1, $2, %s)`,
			sello.ColumnList(), sello.Placeholders(3))
		if _, err := tx.Exec(ctx, q, append([]any{id, b.Slug}, sello.Values...)...); err != nil {
			return traducirPg(err)
		}

		if err := insertarVersion(ctx, tx, id, b); err != nil {
			return err
		}

		creada, err = obtenerCon(ctx, tx, id)
		return err
	})
	return creada, err
}

// guardar sube la version de la pagina en la misma sentencia que compara la
// anterior, y dentro de la transaccion que crea la version nueva.
//
// El UPDATE deja la fila de la pagina bloqueada hasta el commit. Eso es lo que
// hace seguro calcular el numero de version con `max(number) + 1`: otro
// guardado de la misma pagina espera al bloqueo, y cuando entra ya no le cuadra
// la version y sale con 409.
func (r *Repo) guardar(ctx context.Context, id uuid.UUID, version int, b borrador) (Pagina, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Pagina{}, err
	}

	var guardada Pagina
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := fmt.Sprintf(`update pages set slug = $1, version = version + 1, %s
		                   where id = $2 and version = $3`, sello.Assignments(4))
		tag, err := tx.Exec(ctx, q, append([]any{b.Slug, id, version}, sello.Values...)...)
		if err != nil {
			return traducirPg(err)
		}
		if tag.RowsAffected() == 0 {
			// Cero filas dice "no cuadro", no por que. 404 y 409 llevan al
			// usuario a cosas distintas.
			return porQueNoCuadro(ctx, tx, id)
		}

		if err := insertarVersion(ctx, tx, id, b); err != nil {
			return err
		}

		guardada, err = obtenerCon(ctx, tx, id)
		return err
	})
	return guardada, err
}

func insertarVersion(ctx context.Context, tx pgx.Tx, pagina uuid.UUID, b borrador) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	sello, err := audit.ForAppend(ctx)
	if err != nil {
		return err
	}
	bloques, err := json.Marshal(b.Blocks)
	if err != nil {
		return fmt.Errorf("content: los bloques no se pueden serializar: %w", err)
	}

	// Un solo INSERT ... SELECT: el numero se calcula en la base, con la fila
	// de la pagina ya bloqueada por quien llama.
	q := fmt.Sprintf(`insert into page_versions
		(id, page_id, number, title, seo_title, seo_description, blocks, note, %s)
		select $1, $2, coalesce(max(number), 0) + 1, $3, $4, $5, $6, $7, %s
		  from page_versions where page_id = $2`,
		sello.ColumnList(), sello.Placeholders(8))

	args := append([]any{id, pagina, b.Title, b.SeoTitle, b.SeoDescription, bloques, b.Note}, sello.Values...)
	_, err = tx.Exec(ctx, q, args...)
	return err
}

// publicar mueve el puntero y nada mas. Comprueba que la version sea de esta
// pagina en el mismo UPDATE —la llave compuesta de la migracion lo impediria
// igual, pero con un error de base en vez de uno que nombra el campo—.
//
// No sube `version`: publicar no toca el borrador, y quien lo esta editando no
// tiene por que recibir un 409.
func (r *Repo) publicar(ctx context.Context, id, versionID uuid.UUID) (Pagina, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Pagina{}, err
	}

	var publicada Pagina
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := fmt.Sprintf(`update pages p set published_version_id = v.id, %s
		                    from page_versions v
		                   where p.id = $1 and v.id = $2 and v.page_id = p.id`,
			sello.Assignments(3))
		tag, err := tx.Exec(ctx, q, append([]any{id, versionID}, sello.Values...)...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			if err := existe(ctx, tx, id); err != nil {
				return err
			}
			return errVersionAjena
		}

		publicada, err = obtenerCon(ctx, tx, id)
		return err
	})
	return publicada, err
}

// borrar se lleva las versiones por el `on delete cascade`.
func (r *Repo) borrar(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `delete from pages where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNoExiste
	}
	return nil
}

func porQueNoCuadro(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	if err := existe(ctx, tx, id); err != nil {
		return err
	}
	return errVersion
}

func existe(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var ok bool
	err := tx.QueryRow(ctx, `select true from pages where id = $1`, id).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNoExiste
	}
	return err
}

// leerBloques nunca devuelve nil: una pagina sin bloques sale como `[]`. Un
// cliente que hace `blocks.map(...)` revienta con null.
func leerBloques(crudo []byte) ([]Bloque, error) {
	bloques := []Bloque{}
	if err := json.Unmarshal(crudo, &bloques); err != nil {
		return nil, fmt.Errorf("content: una version tiene bloques que no son JSON: %w", err)
	}
	return bloques, nil
}

func uuidONulo(u pgtype.UUID) *openapi_types.UUID {
	if !u.Valid {
		return nil
	}
	id := openapi_types.UUID(u.Bytes)
	return &id
}

// traducirPg convierte la violacion de unicidad del slug en el sentinela del
// paquete. Se mira el nombre de la restriccion y no solo el codigo: la unica
// otra unicidad —el numero de version— no es culpa del cliente, y reportarla
// como "slug ocupado" mandaria a buscar el error donde no esta.
func traducirPg(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "pages_slug_key" {
		return errSlugOcupado
	}
	return err
}
