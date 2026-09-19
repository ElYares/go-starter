package identity

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// Lo que el dashboard de cuentas le pide al service (HU-018). Las reglas duras
// —la invariante del ultimo superadmin, revocar al deshabilitar— viven en el
// SQL del repositorio, que es el unico sitio donde son atomicas. Aqui se valida
// y se traduce.
//
// No hay filtro por propiedad, y es deliberado: una cuenta no es de quien la
// creo. Lo que decide quien la toca es el permiso de la ruta, igual que en
// content. Ver docs/04-reglas-de-crud.md seccion 5.

func (s *Service) Cuentas(ctx context.Context, q url.Values) (paging.Page[UsuarioConRoles], error) {
	p, prob := listadoDeCuentas.Parse(q)
	if prob != nil {
		return paging.Page[UsuarioConRoles]{}, prob
	}

	items, total, err := s.repo.cuentas(ctx, p)
	if err != nil {
		return paging.Page[UsuarioConRoles]{}, err
	}
	return paging.NewPage(items, p, total), nil
}

// CrearCuenta es el alta desde el dashboard: sin roles, siempre. Repartirlos
// es otro permiso y otra ruta, y que este camino no tenga donde recibirlos es
// lo que impide que `identity.user.write` sirva para darlos.
func (s *Service) CrearCuenta(ctx context.Context, email, password, displayName string) (UsuarioConRoles, error) {
	u, err := s.Crear(ctx, UsuarioNuevo{Email: email, Password: password, DisplayName: displayName})
	if err != nil {
		return UsuarioConRoles{}, err
	}
	return UsuarioConRoles{Usuario: u, Roles: []string{}}, nil
}

// Cuenta lee una cuenta con sus roles.
func (s *Service) Cuenta(ctx context.Context, id string) (UsuarioConRoles, error) {
	u, err := s.repo.porID(ctx, id)
	if err != nil {
		return UsuarioConRoles{}, traducirEstado(err)
	}
	return s.conRoles(ctx, u)
}

// Editar cambia correo y nombre. Valida lo mismo que el alta: un correo que no
// pasaria al crear tampoco puede entrar por un guardado.
func (s *Service) Editar(ctx context.Context, id string, version int, m ModificacionDeUsuario) (UsuarioConRoles, error) {
	m.Email = normalizarEmail(m.Email)
	if prob := validarEmail(m.Email); prob != nil {
		return UsuarioConRoles{}, prob
	}
	if prob := validarDisplayName(m.DisplayName); prob != nil {
		return UsuarioConRoles{}, prob
	}
	m.DisplayName = strings.TrimSpace(m.DisplayName)

	u, err := s.repo.editar(ctx, id, version, m)
	if err != nil {
		return UsuarioConRoles{}, traducirEstado(err)
	}
	return s.conRoles(ctx, u)
}

// DeshabilitarCuenta es Deshabilitar con los roles, que es lo que devuelve la
// ruta. Deshabilitar sigue existiendo aparte porque lo usan la siembra y sus
// pruebas, que no necesitan la segunda lectura.
func (s *Service) DeshabilitarCuenta(ctx context.Context, id string) (UsuarioConRoles, error) {
	u, err := s.Deshabilitar(ctx, id)
	if err != nil {
		return UsuarioConRoles{}, err
	}
	return s.conRoles(ctx, u)
}

func (s *Service) Habilitar(ctx context.Context, id string) (UsuarioConRoles, error) {
	u, err := s.repo.habilitar(ctx, id)
	if err != nil {
		return UsuarioConRoles{}, traducirEstado(err)
	}
	return s.conRoles(ctx, u)
}

// DarRol es AsignarRol visto desde la ruta, donde el rol va en la URL: un rol
// que no existe es un recurso que no existe, `404`. AsignarRol lo contesta con
// `409` porque ahi el rol llega en un cuerpo —el alta de la siembra— y lo que
// esta mal es un campo, no la direccion.
func (s *Service) DarRol(ctx context.Context, userID, rol string) error {
	err := s.repo.asignarRol(ctx, userID, rol)
	if errors.Is(err, errRolNoExiste) {
		return httpx.NotFound()
	}
	return traducirEstado(err)
}

func (s *Service) RolesConPermisos(ctx context.Context, q url.Values) (paging.Page[RolConPermisos], error) {
	p, prob := listadoDeRoles.Parse(q)
	if prob != nil {
		return paging.Page[RolConPermisos]{}, prob
	}

	items, total, err := s.repo.catalogoDeRoles(ctx, p)
	if err != nil {
		return paging.Page[RolConPermisos]{}, err
	}
	return paging.NewPage(items, p, total), nil
}

func (s *Service) conRoles(ctx context.Context, u Usuario) (UsuarioConRoles, error) {
	roles, err := s.repo.rolesDe(ctx, u.ID)
	if err != nil {
		return UsuarioConRoles{}, err
	}
	return UsuarioConRoles{Usuario: u, Roles: roles}, nil
}
