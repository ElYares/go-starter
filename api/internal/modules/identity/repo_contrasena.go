package identity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// Las consultas de la contrasena temporal (HU-019): pedirla desde el login,
// asignarla, y cambiar la propia.

// La mas vieja primero: es la que lleva mas tiempo esperando.
var listadoDeSolicitudes = paging.NewSpec("id").
	DefaultOrder(paging.Order{Column: "created_at"})

// pedirContrasena deja una solicitud pendiente, o nada.
//
// **Es UNA sentencia haga lo que haga.** Si el correo no existe, si la cuenta
// esta deshabilitada o si ya hay una pendiente, no inserta nada y no dice por
// que: el `select` no devuelve fila o el indice unico parcial descarta la
// repetida. Separarlo en "buscar la cuenta" y "insertar" haria que un correo
// registrado tardara una consulta mas que uno inventado, y eso se mide.
func (r *Repo) pedirContrasena(ctx context.Context, id, email, ip, userAgent string) error {
	var ipONulo, uaONulo *string
	if ip != "" {
		ipONulo = &ip
	}
	if userAgent != "" {
		uaONulo = &userAgent
	}

	_, err := r.pool.Exec(ctx,
		`insert into password_reset_requests (id, user_id, ip, user_agent)
		 select $1, u.id, $3, $4 from users u where u.email = $2 and u.enabled
		 on conflict (user_id) where resolved_at is null do nothing`,
		id, email, ipONulo, uaONulo)
	return err
}

// asignarContrasena pone una contrasena temporal, revoca las sesiones de la
// cuenta y da por atendidas sus solicitudes, todo en una transaccion.
//
// **La regla de poder va en el WHERE de la sentencia que escribe**: la cuenta
// no puede tener ningun permiso que el actor no tenga. Comprobarlo en Go con
// dos lecturas y escribir despues deja una ventana en la que a la cuenta le
// dan un rol entre la comprobacion y la escritura. Es la misma razon que la
// invariante del ultimo superadmin.
func (r *Repo) asignarContrasena(ctx context.Context, actorID, userID, hash string) error {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := fmt.Sprintf(`update users
		       set password_hash = $1, must_change_password = true, version = version + 1, %s
		     where id = $2
		       and not exists (
		           select 1
		             from user_roles ur
		             join role_permissions rp on rp.role_id = ur.role_id
		            where ur.user_id = users.id
		              and rp.permission_key not in (
		                  select rp2.permission_key
		                    from user_roles ur2
		                    join role_permissions rp2 on rp2.role_id = ur2.role_id
		                   where ur2.user_id = $3))`,
		sello.Assignments(4))

	tag, err := tx.Exec(ctx, q, append([]any{hash, userID, actorID}, sello.Values...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Cero filas: o no existe, o tiene mas poder. El 404 y el 403 le piden
		// cosas distintas a quien la asigna.
		if _, err := r.porID(ctx, userID); err != nil {
			return err
		}
		return errMasPoder
	}

	if _, err := tx.Exec(ctx, revocarLasDe, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`update password_reset_requests set resolved_at = now(), resolved_by = $2
		  where user_id = $1 and resolved_at is null`, userID, actorID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// cambiarContrasena pone la que eligio la propia cuenta: deja de ser temporal
// y se revocan TODAS sus sesiones. La que hizo el cambio sigue viva porque el
// service emite una nueva despues; ver Service.CambiarContrasena.
func (r *Repo) cambiarContrasena(ctx context.Context, userID, hash string) error {
	sello, err := audit.ForUpdate(ctx)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := fmt.Sprintf(`update users
		       set password_hash = $1, must_change_password = false, version = version + 1, %s
		     where id = $2`, sello.Assignments(3))

	tag, err := tx.Exec(ctx, q, append([]any{hash, userID}, sello.Values...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNoExiste
	}

	if _, err := tx.Exec(ctx, revocarLasDe, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const fuenteDeSolicitudes = `(
	select s.id, s.user_id, u.email, u.display_name, s.created_at
	  from password_reset_requests s
	  join users u on u.id = s.user_id
	 where s.resolved_at is null
) solicitudes`

func (r *Repo) solicitudes(ctx context.Context, p paging.Params) ([]SolicitudPendiente, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `select count(*) from `+fuenteDeSolicitudes).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := r.pool.Query(ctx,
		`select id, user_id, email, display_name, created_at from `+fuenteDeSolicitudes+` `+
			p.OrderBy()+` limit $1 offset $2`, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (SolicitudPendiente, error) {
		var s SolicitudPendiente
		err := row.Scan(&s.ID, &s.UserID, &s.Email, &s.DisplayName, &s.CreatedAt)
		return s, err
	})
	return items, total, err
}
