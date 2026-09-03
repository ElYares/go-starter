package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// La lista blanca de este recurso, declarada junto a su repositorio y no en un
// mapa global. El desempate es `key`, que es la clave primaria: sin el, dos
// filas con el mismo updated_at se repiten o se pierden entre paginas.
//
// `value` no es ordenable ni filtrable: es jsonb libre, y ordenar por el
// significaria ordenar por su representacion textual, que no es lo que nadie
// espera al pedirlo.
var listado = paging.NewSpec("key").
	SortBy("key", "key").
	SortBy("updatedAt", "updated_at").
	FilterBy("isPublic", "is_public", paging.Bool)

// Errores del repositorio. Son sentinelas y no *httpx.Problem a proposito: el
// repositorio no decide codigos HTTP. Quien traduce es el service.
var (
	errNoExiste = errors.New("settings: la clave no existe")
	errYaExiste = errors.New("settings: la clave ya existe")
	errVersion  = errors.New("settings: la version no coincide")
)

// columnas es la proyeccion unica. Una sola constante evita que la consulta de
// listar y la de leer devuelvan formas distintas del mismo recurso.
const columnas = `key, value, is_public, version, updated_at, updated_by`

type Repo struct {
	pool *pgxpool.Pool
}

// listar trae la configuracion entera, sin paginar. Es lo que consume el
// renderizado en servidor de la landing en cada carga.
//
// `soloPublicas` se resuelve EN LA CONSULTA, no filtrando en Go despues de
// traer todo: una clave privada que no sale de la base no se puede filtrar mal
// por descuido mas arriba.
func (r *Repo) listar(ctx context.Context, soloPublicas bool) ([]Setting, error) {
	const q = `select ` + columnas + `
		  from settings
		 where (not $1::boolean) or is_public
		 order by key asc`

	rows, err := r.pool.Query(ctx, q, soloPublicas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, escanear)
}

// pagina aplica el molde: el orden y el filtro ya vienen resueltos a columnas
// reales por paging, y el limite y el desplazamiento van en la consulta.
//
// Son dos consultas y no una transaccion: entre el conteo y la pagina puede
// entrar una escritura y dejar el total desfasado por uno. En una tabla de
// configuracion que se escribe a mano desde un dashboard, abrir una
// transaccion por cada listado cuesta mas de lo que ese desfase vale.
func (r *Repo) pagina(ctx context.Context, p paging.Params) ([]Setting, int64, error) {
	where, args := p.Where(1)

	var total int64
	if err := r.pool.QueryRow(ctx, `select count(*) from settings `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	q := fmt.Sprintf(`select %s from settings %s %s limit $%d offset $%d`,
		columnas, where, p.OrderBy(), len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, escanear)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repo) obtener(ctx context.Context, key string) (Setting, error) {
	const q = `select ` + columnas + ` from settings where key = $1`

	rows, err := r.pool.Query(ctx, q, key)
	if err != nil {
		return Setting{}, err
	}
	defer rows.Close()

	s, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		return Setting{}, errNoExiste
	}
	return s, err
}

// crear no escribe una sola columna de auditoria a mano: las columnas, sus
// marcadores y sus valores salen de platform/audit.
//
// La fila y su auditoria se escriben en UNA sentencia, que en Postgres ya es
// una transaccion. Por eso no hay un BEGIN explicito aqui y aun asi se cumple
// docs/04-reglas-de-crud.md seccion 4: no existe un estado intermedio en el que
// la fila este y su sello no.
func (r *Repo) crear(ctx context.Context, s Setting) (Setting, error) {
	sello, err := audit.ForInsert(ctx)
	if err != nil {
		return Setting{}, err
	}

	q := fmt.Sprintf(`insert into settings (key, value, is_public, %s)
		     values ($1, $2, $3, %s)
		  returning %s`,
		sello.ColumnList(), sello.Placeholders(4), columnas)

	args := append([]any{s.Key, s.Value, s.IsPublic}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Setting{}, traducir(err)
	}
	defer rows.Close()

	creada, err := pgx.CollectExactlyOneRow(rows, escanear)
	return creada, traducir(err)
}

// actualizar sube la version en la misma sentencia que compara la anterior. Es
// lo que hace la comprobacion atomica: leer la version, decidir en Go y
// escribir despues deja una ventana en la que otro guarda entre medias, que es
// exactamente el bug que el `version` existe para cerrar.
func (r *Repo) actualizar(ctx context.Context, s Setting) (Setting, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Setting{}, err
	}

	q := fmt.Sprintf(`update settings
		       set value = $1, is_public = $2, version = version + 1, %s
		     where key = $3 and version = $4
		  returning %s`,
		sello.Assignments(5), columnas)

	args := append([]any{s.Value, s.IsPublic, s.Key, s.Version}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Setting{}, err
	}
	defer rows.Close()

	actualizada, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		// Cero filas dice "no cuadro", no por que. Distinguir las dos causas
		// importa: 404 y 409 llevan al usuario a cosas distintas.
		return Setting{}, r.porQueNoCuadro(ctx, s.Key)
	}
	return actualizada, err
}

func (r *Repo) porQueNoCuadro(ctx context.Context, key string) error {
	var existe bool
	err := r.pool.QueryRow(ctx, `select true from settings where key = $1`, key).Scan(&existe)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNoExiste
	}
	if err != nil {
		return err
	}
	return errVersion
}

func escanear(row pgx.CollectableRow) (Setting, error) {
	var (
		s  Setting
		by pgtype.UUID
	)
	if err := row.Scan(&s.Key, &s.Value, &s.IsPublic, &s.Version, &s.UpdatedAt, &by); err != nil {
		return Setting{}, err
	}
	if by.Valid {
		quien := by.String()
		s.UpdatedBy = &quien
	}
	return s, nil
}

// traducir convierte la violacion de unicidad de Postgres en el sentinela del
// paquete. Comprobar antes con un select y crear despues seria una carrera: la
// unica comprobacion fiable es la de la base.
func traducir(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errYaExiste
	}
	return err
}
