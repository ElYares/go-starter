package identity

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Los handlers del dashboard de cuentas (HU-018). Los nombres y las firmas los
// dicta ServerInterface; si se renombra una operacion en api/openapi.yaml y no
// aqui, la afirmacion de module.go deja de compilar.
//
// Ninguno tiene un `if` de negocio: decodifican, llaman al service y traducen
// el dominio a la forma de cable. La traduccion vive aqui y no en el service
// porque `Cuenta` es un tipo del contrato, y el service no sabe que existe.

func (m *Module) ListarCuentas(w http.ResponseWriter, r *http.Request, _ ListarCuentasParams) {
	pagina, err := m.svc.Cuentas(r.Context(), r.URL.Query())
	if err != nil {
		escribirError(w, r, err)
		return
	}

	cuentas := make([]Cuenta, len(pagina.Content))
	for i, c := range pagina.Content {
		if cuentas[i], err = aCuenta(c); err != nil {
			escribirError(w, r, err)
			return
		}
	}
	httpx.WriteJSON(w, r, http.StatusOK, CuentasPage{Content: cuentas, Page: PageMeta(pagina.Page)})
}

func (m *Module) CrearCuenta(w http.ResponseWriter, r *http.Request) {
	var nueva CuentaNueva
	if prob := httpx.DecodeJSON(w, r, &nueva); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	creada, err := m.svc.CrearCuenta(r.Context(), nueva.Email, nueva.Password, nueva.DisplayName)
	if err != nil {
		escribirError(w, r, err)
		return
	}

	cuenta, err := aCuenta(creada)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	w.Header().Set("ETag", httpx.ETag(cuenta.Version))
	httpx.Created(w, r, "/api/v1/users/"+cuenta.Id.String(), cuenta)
}

func (m *Module) LeerCuenta(w http.ResponseWriter, r *http.Request, id CuentaId) {
	m.responderCuenta(w, r, func(ctx context.Context) (UsuarioConRoles, error) {
		return m.svc.Cuenta(ctx, id.String())
	})
}

// GuardarCuenta exige la version que el cliente creia estar editando. Que
// `If-Match` venga lo garantiza el codigo generado; su forma se comprueba aqui.
func (m *Module) GuardarCuenta(w http.ResponseWriter, r *http.Request, id CuentaId, params GuardarCuentaParams) {
	version, prob := httpx.VersionFromIfMatch(params.IfMatch)
	if prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	var mod CuentaModificacion
	if prob := httpx.DecodeJSON(w, r, &mod); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}

	m.responderCuenta(w, r, func(ctx context.Context) (UsuarioConRoles, error) {
		return m.svc.Editar(ctx, id.String(), version, ModificacionDeUsuario{
			Email: mod.Email, DisplayName: mod.DisplayName,
		})
	})
}

func (m *Module) DeshabilitarCuenta(w http.ResponseWriter, r *http.Request, id CuentaId) {
	m.responderCuenta(w, r, func(ctx context.Context) (UsuarioConRoles, error) {
		return m.svc.DeshabilitarCuenta(ctx, id.String())
	})
}

func (m *Module) HabilitarCuenta(w http.ResponseWriter, r *http.Request, id CuentaId) {
	m.responderCuenta(w, r, func(ctx context.Context) (UsuarioConRoles, error) {
		return m.svc.Habilitar(ctx, id.String())
	})
}

func (m *Module) AsignarRol(w http.ResponseWriter, r *http.Request, id CuentaId, role RolKey) {
	if err := m.svc.DarRol(r.Context(), id.String(), role); err != nil {
		escribirError(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (m *Module) QuitarRol(w http.ResponseWriter, r *http.Request, id CuentaId, role RolKey) {
	if err := m.svc.QuitarRol(r.Context(), id.String(), role); err != nil {
		escribirError(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (m *Module) ListarRoles(w http.ResponseWriter, r *http.Request, _ ListarRolesParams) {
	pagina, err := m.svc.RolesConPermisos(r.Context(), r.URL.Query())
	if err != nil {
		escribirError(w, r, err)
		return
	}

	roles := make([]Rol, len(pagina.Content))
	for i, rol := range pagina.Content {
		permisos := make([]PermisoConcedido, len(rol.Permisos))
		for j, p := range rol.Permisos {
			permisos[j] = PermisoConcedido{Key: p.Key, Description: p.Desc, Sensitive: p.Sensitive}
		}
		roles[i] = Rol{Key: rol.Key, Name: rol.Name, Permissions: permisos}
	}
	httpx.WriteJSON(w, r, http.StatusOK, RolesPage{Content: roles, Page: PageMeta(pagina.Page)})
}

// responderCuenta es la salida comun de las operaciones que devuelven una
// cuenta: cuerpo y `ETag`, siempre juntos. Que el ETag salga en un solo sitio
// es lo que impide que una de las cinco se olvide de mandarlo y el editor se
// quede guardando con una version vieja.
func (m *Module) responderCuenta(w http.ResponseWriter, r *http.Request, op func(context.Context) (UsuarioConRoles, error)) {
	u, err := op(r.Context())
	if err != nil {
		escribirError(w, r, err)
		return
	}

	cuenta, err := aCuenta(u)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	w.Header().Set("ETag", httpx.ETag(cuenta.Version))
	httpx.WriteJSON(w, r, http.StatusOK, cuenta)
}

// aCuenta traduce el dominio a la forma de cable. Los ids salen de columnas
// `uuid`, asi que el parseo no puede fallar; se comprueba igual, por la misma
// razon que en MiPerfil.
func aCuenta(u UsuarioConRoles) (Cuenta, error) {
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return Cuenta{}, fmt.Errorf("identity: el id %q no es un uuid: %w", u.ID, err)
	}

	var por *uuid.UUID
	if u.UpdatedBy != nil {
		p, err := uuid.Parse(*u.UpdatedBy)
		if err != nil {
			return Cuenta{}, fmt.Errorf("identity: updated_by %q no es un uuid: %w", *u.UpdatedBy, err)
		}
		por = &p
	}

	return Cuenta{
		Id:          id,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Enabled:     u.Enabled,
		Roles:       oVacio(u.Roles),
		Version:     u.Version,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		UpdatedBy:   por,
	}, nil
}
