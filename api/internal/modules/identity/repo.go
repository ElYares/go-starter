package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// columnas es la proyeccion unica del usuario. Una sola constante evita que dos
// consultas devuelvan formas distintas del mismo recurso.
//
// password_hash no esta: sale solo por la consulta de autenticacion, que lo
// pide aparte y con nombre. Asi no puede colarse en un listado por descuido.
const columnas = `id, email, display_name, enabled, dev_seed, version`

type Repo struct {
	pool *pgxpool.Pool
}

func (r *Repo) crear(ctx context.Context, u Usuario, hash string) (Usuario, error) {
	sello, err := audit.ForInsert(ctx)
	if err != nil {
		return Usuario{}, err
	}

	q := fmt.Sprintf(`insert into users (id, email, password_hash, display_name, enabled, dev_seed, %s)
		     values ($1, $2, $3, $4, $5, $6, %s)
		  returning %s`,
		sello.ColumnList(), sello.Placeholders(7), columnas)

	args := append([]any{u.ID, u.Email, hash, u.DisplayName, u.Enabled, u.DevSeed}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Usuario{}, traducir(err)
	}
	defer rows.Close()

	creado, err := pgx.CollectExactlyOneRow(rows, escanear)
	return creado, traducir(err)
}

func (r *Repo) porID(ctx context.Context, id string) (Usuario, error) {
	rows, err := r.pool.Query(ctx, `select `+columnas+` from users where id = $1`, id)
	if err != nil {
		return Usuario{}, err
	}
	defer rows.Close()

	u, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, errNoExiste
	}
	return u, err
}

// paraAutenticar es la unica consulta que saca el hash de la base.
//
// La comparacion de correo la hace `citext`: no hay lower() aqui porque no
// puede haberlo en el indice tampoco, y una comparacion insensible en Go contra
// un indice sensible es el bug clasico de "no encuentra al usuario que existe".
func (r *Repo) paraAutenticar(ctx context.Context, email string) (Usuario, string, error) {
	var (
		u    Usuario
		hash string
	)
	err := r.pool.QueryRow(ctx,
		`select `+columnas+`, password_hash from users where email = $1`, email).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.Enabled, &u.DevSeed, &u.Version, &hash)

	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, "", errNoExiste
	}
	return u, hash, err
}

// actualizarCredencial la usa `cmd/seed` para poder correr dos veces. Reescribe
// la contrasena y vuelve a habilitar, conservando el id: si generara uno nuevo,
// cada seed dejaria huerfanas las filas que apuntaban al anterior.
func (r *Repo) actualizarCredencial(ctx context.Context, email, hash, displayName string) (Usuario, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Usuario{}, err
	}

	q := fmt.Sprintf(`update users
		       set password_hash = $1, display_name = $2, enabled = true,
		           dev_seed = true, version = version + 1, %s
		     where email = $3
		  returning %s`,
		sello.Assignments(4), columnas)

	args := append([]any{hash, displayName, email}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Usuario{}, err
	}
	defer rows.Close()

	u, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, errNoExiste
	}
	return u, err
}

// permisosDe resuelve el actor: los permisos efectivos de un usuario, por sus
// roles. Es lo que rellena rbac.Actor cuando la sesion llegue.
func (r *Repo) permisosDe(ctx context.Context, userID string) ([]string, error) {
	const q = `select distinct rp.permission_key
		     from user_roles ur
		     join role_permissions rp on rp.role_id = ur.role_id
		    where ur.user_id = $1
		    order by rp.permission_key`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *Repo) rolesDe(ctx context.Context, userID string) ([]string, error) {
	const q = `select ro.key
		     from user_roles ur
		     join roles ro on ro.id = ur.role_id
		    where ur.user_id = $1
		    order by ro.key`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// asignarRol es idempotente: asignar dos veces el mismo rol no es un error, es
// el mismo estado.
func (r *Repo) asignarRol(ctx context.Context, userID, rol string) error {
	const q = `insert into user_roles (user_id, role_id)
		   select $1, id from roles where key = $2
		   on conflict do nothing`

	tag, err := r.pool.Exec(ctx, q, userID, rol)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		// Cero filas dice "no inserte", no por que. O el rol no existe, o el
		// usuario ya lo tenia: la primera es un error y la segunda no.
		return r.porQueNoAsigne(ctx, userID, rol)
	}
	return nil
}

func (r *Repo) porQueNoAsigne(ctx context.Context, userID, rol string) error {
	var yaLoTenia bool
	err := r.pool.QueryRow(ctx,
		`select exists (
		     select 1 from user_roles ur join roles ro on ro.id = ur.role_id
		      where ur.user_id = $1 and ro.key = $2)`, userID, rol).Scan(&yaLoTenia)
	if err != nil {
		return err
	}
	if yaLoTenia {
		return nil
	}
	return errRolNoExiste
}

// quitarRol comprueba la invariante EN LA MISMA SENTENCIA que borra.
//
// Contar los superadmins en Go y borrar despues deja una ventana en la que otra
// peticion quita el otro rol entre medias, y las dos pasan la comprobacion. El
// resultado es una instalacion sin ningun superadmin, que es exactamente lo que
// esta invariante existe para impedir.
func (r *Repo) quitarRol(ctx context.Context, userID, rol string) error {
	q := fmt.Sprintf(`delete from user_roles
		    where user_id = $1
		      and role_id = (select id from roles where key = $2)
		      and (
		            $2::text <> $3::text
		         or not exists (select 1 from users u where u.id = $1 and u.enabled)
		         or %s
		      )`, hayOtroSuperadminHabilitado(1, 3))

	tag, err := r.pool.Exec(ctx, q, userID, rol, RolSuperadmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return r.porQueNoQuite(ctx, userID, rol)
	}
	return nil
}

func (r *Repo) porQueNoQuite(ctx context.Context, userID, rol string) error {
	var loTiene bool
	err := r.pool.QueryRow(ctx,
		`select exists (
		     select 1 from user_roles ur join roles ro on ro.id = ur.role_id
		      where ur.user_id = $1 and ro.key = $2)`, userID, rol).Scan(&loTiene)
	if err != nil {
		return err
	}
	if !loTiene {
		return errRolNoAsignado
	}
	// Lo tiene y aun asi no se borro: el unico filtro que queda es la
	// invariante.
	return errUltimoSuperadmin
}

// hayOtroSuperadminHabilitado es la mitad compartida de la invariante. Se
// genera en vez de escribirse dos veces para que las dos operaciones que la
// aplican —quitar el rol y deshabilitar— no se separen el dia que una se toque
// y la otra no.
//
// Recibe los ordinales porque cada consulta los tiene en otro sitio, y un
// marcador que el SQL no referencia hace que Postgres se niegue a preparar la
// sentencia: "could not determine data type of parameter". Mandar un nil de
// relleno no arregla eso.
func hayOtroSuperadminHabilitado(usuario, rolSuper int) string {
	return fmt.Sprintf(`exists (
	select 1
	  from user_roles ur
	  join roles ro on ro.id = ur.role_id
	  join users otro on otro.id = ur.user_id
	 where ro.key = $%d::text and otro.enabled and otro.id <> $%d)`, rolSuper, usuario)
}

// deshabilitar aplica la misma invariante, y por la misma razon atomica.
func (r *Repo) deshabilitar(ctx context.Context, userID string) (Usuario, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Usuario{}, err
	}

	q := fmt.Sprintf(`update users
		       set enabled = false, version = version + 1, %s
		     where id = $1
		       and enabled
		       and (
		             not exists (
		                 select 1 from user_roles ur
		                   join roles ro on ro.id = ur.role_id
		                  where ur.user_id = users.id and ro.key = $2::text)
		          or %s
		       )
		  returning %s`,
		sello.Assignments(3), hayOtroSuperadminHabilitado(1, 2), columnas)

	args := append([]any{userID, RolSuperadmin}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Usuario{}, err
	}
	defer rows.Close()

	u, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.porQueNoDeshabilite(ctx, userID)
	}
	return u, err
}

func (r *Repo) porQueNoDeshabilite(ctx context.Context, userID string) (Usuario, error) {
	u, err := r.porID(ctx, userID)
	if err != nil {
		return Usuario{}, err
	}
	if !u.Enabled {
		// Ya estaba deshabilitado. Deshabilitarlo otra vez es el mismo estado,
		// no un conflicto: se devuelve la fila como esta.
		return u, nil
	}
	// Existe, esta habilitado y aun asi no se actualizo: el unico filtro que
	// queda en el WHERE es la invariante.
	return Usuario{}, errUltimoSuperadmin
}

// SembrarPermisos reconcilia el catalogo con lo que declaran los modulos, y
// reparte lo que corresponde a cada uno de los dos roles que no se configuran.
//
// Las cuatro sentencias van en UNA transaccion porque entre el borrado y las
// concesiones hay un instante en el que el superadmin no tiene un permiso que
// si deberia tener; sin la transaccion, una peticion que caiga justo ahi recibe
// un 403 que nadie puede reproducir despues.
func (r *Repo) SembrarPermisos(ctx context.Context, perms []rbac.Permission) error {
	claves := make([]string, len(perms))
	descripciones := make([]string, len(perms))
	sensibles := make([]bool, len(perms))
	var noSensibles []string

	for i, p := range perms {
		claves[i] = p.Key
		descripciones[i] = p.Desc
		sensibles[i] = p.Sensitive
		if !p.Sensitive {
			noSensibles = append(noSensibles, p.Key)
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Alta y actualizacion en una: volver a arrancar no duplica, y cambiar la
	// descripcion de un permiso se refleja sin migracion.
	const upsert = `insert into permissions (key, description, sensitive)
			select k, d, s from unnest($1::text[], $2::text[], $3::boolean[]) as t(k, d, s)
			on conflict (key) do update
			   set description = excluded.description,
			       sensitive   = excluded.sensitive`
	if _, err := tx.Exec(ctx, upsert, claves, descripciones, sensibles); err != nil {
		return err
	}

	// Y la otra mitad: lo que ya nadie declara se va. Es lo que impide que
	// borrar un modulo deje permisos huerfanos nombrando algo que no existe.
	// El `on delete cascade` de role_permissions se lleva sus concesiones.
	if _, err := tx.Exec(ctx, `delete from permissions where key <> all($1::text[])`, claves); err != nil {
		return err
	}

	// El superadmin es, por definicion, todos los permisos declarados. Sin
	// esto, el permiso de un modulo nuevo nace inalcanzable: nadie lo tiene, y
	// la pantalla para concederlo tambien lo exige.
	//
	// No hace falta borrarle nada: el `delete` de arriba ya arrastro por
	// cascada lo que dejo de existir, asi que su lista es exacta.
	const alSuperadmin = `insert into role_permissions (role_id, permission_key)
			select ro.id, p.key from roles ro cross join permissions p
			 where ro.key = $1
			on conflict do nothing`
	if _, err := tx.Exec(ctx, alSuperadmin, RolSuperadmin); err != nil {
		return err
	}

	// El admin arranca con lo que ningun modulo marco como sensible, y SOLO si
	// todavia no tiene ninguna concesion. Es un punto de partida para un fork
	// recien clonado, no una regla permanente.
	//
	// La condicion es lo que impide que la siembra pelee con el dashboard:
	// quitarle un permiso al admin desde la pantalla de roles tiene que quedar
	// quitado, y no volver en el siguiente despliegue. A partir del primer
	// arranque, sus concesiones son datos que se editan.
	const alAdmin = `insert into role_permissions (role_id, permission_key)
			select ro.id, k
			  from roles ro cross join unnest($2::text[]) as t(k)
			 where ro.key = $1
			   and not exists (select 1 from role_permissions rp where rp.role_id = ro.id)
			on conflict do nothing`
	if _, err := tx.Exec(ctx, alAdmin, RolAdmin, noSensibles); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func escanear(row pgx.CollectableRow) (Usuario, error) {
	var u Usuario
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Enabled, &u.DevSeed, &u.Version)
	return u, err
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

// guardarRefresh deja constancia de una sesion abierta.
//
// Guarda el SHA-256 del token, nunca el token: una fuga de la base no debe
// entregar sesiones activas. La columna es `bytea` y no `text` porque un hash
// son bytes; guardarlo en hexadecimal invita a que alguien compare cadenas con
// mayusculas distintas y no encuentre la fila.
//
// `ip` puede llegar vacia si la peticion no trae con que resolverla. Va como
// NULL y no como cadena vacia: `inet` no acepta ” y el insert entero fallaria,
// tumbando un login que por lo demas estaba bien.
func (r *Repo) guardarRefresh(ctx context.Context, s SesionNueva) error {
	var ip *string
	if s.IP != "" {
		ip = &s.IP
	}

	var ua *string
	if s.UserAgent != "" {
		ua = &s.UserAgent
	}

	_, err := r.pool.Exec(ctx,
		`insert into refresh_tokens (id, user_id, token_hash, expires_at, user_agent, ip)
		      values ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiraEn, ua, ip)

	return traducir(err)
}
