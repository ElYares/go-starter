package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Las consultas del dashboard de cuentas (HU-018). Van aparte de repo.go, que
// es la sesion y la siembra, para que quien busque una no tenga que leer la
// otra.

// Las listas blancas, junto a las consultas que las usan. Las columnas son las
// de la tabla derivada `cuentas`, no las de `users`: asi el filtro y el orden
// no saben nada del join de roles.
var (
	listadoDeCuentas = paging.NewSpec("id").
				SortBy("email", "email").
				SortBy("displayName", "display_name").
				SortBy("createdAt", "created_at").
				FilterBy("enabled", "enabled", paging.Bool).
				DefaultOrder(paging.Order{Column: "email"})

	// La clave es unica: sirve de desempate por si sola.
	listadoDeRoles = paging.NewSpec("key").
			DefaultOrder(paging.Order{Column: "key"})
)

// fuenteDeCuentas agrega los roles en la misma fila. Un `array_agg` y no una
// consulta por cuenta: cien filas serian ciento un viajes a la base.
//
// El `filter` es lo que hace que una cuenta sin roles salga con `{}` y no con
// `{NULL}`, que es lo que da un left join sin coincidencias.
const fuenteDeCuentas = `(
	select u.id, u.email, u.display_name, u.enabled, u.dev_seed, u.version,
	       u.created_at, u.updated_at, u.updated_by,
	       coalesce(array_agg(ro.key order by ro.key) filter (where ro.key is not null), '{}') as roles
	  from users u
	  left join user_roles ur on ur.user_id = u.id
	  left join roles ro on ro.id = ur.role_id
	 group by u.id
) cuentas`

func (r *Repo) cuentas(ctx context.Context, p paging.Params) ([]UsuarioConRoles, int64, error) {
	where, args := p.Where(1)

	var total int64
	if err := r.pool.QueryRow(ctx, `select count(*) from `+fuenteDeCuentas+` `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	q := fmt.Sprintf(`select %s, roles from %s %s %s limit $%d offset $%d`,
		columnas, fuenteDeCuentas, where, p.OrderBy(), len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (UsuarioConRoles, error) {
		var c UsuarioConRoles
		err := row.Scan(append(destinos(&c.Usuario), &c.Roles)...)
		return c, err
	})
	return items, total, err
}

// editar cambia correo y nombre sobre la version que el cliente leyo.
//
// La version va en el WHERE y no en una comprobacion previa: leer, comparar y
// escribir deja una ventana en la que dos guardados pasan la comprobacion y el
// segundo pisa al primero en silencio.
func (r *Repo) editar(ctx context.Context, id string, version int, m ModificacionDeUsuario) (Usuario, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Usuario{}, err
	}

	q := fmt.Sprintf(`update users
		       set email = $1, display_name = $2, version = version + 1, %s
		     where id = $3 and version = $4
		  returning %s`,
		sello.Assignments(5), columnas)

	args := append([]any{m.Email, m.DisplayName, id, version}, sello.Values...)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Usuario{}, traducir(err)
	}
	defer rows.Close()

	u, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		// Cero filas: o no existe, o la version ya no es esa. El 404 y el 409
		// piden cosas distintas al cliente, asi que hay que saber cual.
		if _, err := r.porID(ctx, id); err != nil {
			return Usuario{}, err
		}
		return Usuario{}, errVersion
	}
	return u, traducir(err)
}

// habilitar es idempotente, como deshabilitar: una cuenta ya habilitada se
// devuelve como esta, sin subirle la version.
func (r *Repo) habilitar(ctx context.Context, userID string) (Usuario, error) {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return Usuario{}, err
	}

	q := fmt.Sprintf(`update users
		       set enabled = true, version = version + 1, %s
		     where id = $1 and not enabled
		  returning %s`,
		sello.Assignments(2), columnas)

	rows, err := r.pool.Query(ctx, q, append([]any{userID}, sello.Values...)...)
	if err != nil {
		return Usuario{}, err
	}
	defer rows.Close()

	u, err := pgx.CollectExactlyOneRow(rows, escanear)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.porID(ctx, userID)
	}
	return u, err
}

// catalogoDeRoles devuelve cada rol con los permisos que concede. Los permisos
// van como JSON agregado por la misma razon que los roles de una cuenta: una
// consulta, no una por rol.
func (r *Repo) catalogoDeRoles(ctx context.Context, p paging.Params) ([]RolConPermisos, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `select count(*) from roles`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	q := fmt.Sprintf(`select key, name, permisos from (
		select ro.key, ro.name,
		       coalesce(
		           jsonb_agg(jsonb_build_object(
		               'key', pe.key, 'desc', pe.description, 'sensitive', pe.sensitive)
		               order by pe.key)
		           filter (where pe.key is not null),
		           '[]') as permisos
		  from roles ro
		  left join role_permissions rp on rp.role_id = ro.id
		  left join permissions pe on pe.key = rp.permission_key
		 group by ro.id
	) roles %s limit $1 offset $2`, p.OrderBy())

	rows, err := r.pool.Query(ctx, q, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (RolConPermisos, error) {
		var (
			rol      RolConPermisos
			permisos []permisoGuardado
		)
		if err := row.Scan(&rol.Key, &rol.Name, &permisos); err != nil {
			return RolConPermisos{}, err
		}
		rol.Permisos = make([]rbac.Permission, len(permisos))
		for i, pe := range permisos {
			rol.Permisos[i] = rbac.Permission{Key: pe.Key, Desc: pe.Desc, Sensitive: pe.Sensitive}
		}
		return rol, nil
	})
	return items, total, err
}

// permisoGuardado es la forma del JSON que arma la consulta de roles. Existe
// porque rbac.Permission no tiene etiquetas JSON, y ponerselas en plataforma
// por una consulta de este modulo seria acoplar al reves.
type permisoGuardado struct {
	Key       string `json:"key"`
	Desc      string `json:"desc"`
	Sensitive bool   `json:"sensitive"`
}
