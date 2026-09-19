package identity

import (
	"context"
	"net/url"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/ids"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// La contrasena temporal (HU-019). Las reglas que tienen que ser atomicas —la
// de poder, revocar y atender las solicitudes— viven en el SQL de
// repo_contrasena.go; aqui se valida y se traduce.

// PedidoRecibidoMensaje es la respuesta del pedido, la misma haya cuenta o no.
const PedidoRecibidoMensaje = "Si la cuenta existe, quien administra el sitio vera tu solicitud y te dara una contrasena temporal."

// PedirContrasena deja una solicitud si el correo es de una cuenta habilitada,
// y no dice si lo era. Un correo mal formado si es 400: eso no delata nada.
func (s *Service) PedirContrasena(ctx context.Context, email, ip, userAgent string) error {
	email = normalizarEmail(email)
	if prob := validarEmail(email); prob != nil {
		return prob
	}
	id, err := ids.NewString()
	if err != nil {
		return err
	}
	return s.repo.pedirContrasena(ctx, id, email, ip, userAgent)
}

// AsignarContrasena pone una contrasena temporal a otra cuenta.
//
// A la propia no: quien se la asignara quedaria con la marca de temporal y sin
// permisos hasta volver a cambiarla, que es un rodeo para lo que ya hace
// CambiarContrasena, y ademas sin pedirle la actual.
func (s *Service) AsignarContrasena(ctx context.Context, actorID, userID, password string) error {
	if actorID == userID {
		return httpx.Conflict("Esa es tu cuenta. Tu contrasena se cambia desde \"Cambiar mi contrasena\"")
	}
	if prob := validarPassword(password); prob != nil {
		return prob
	}
	hash, err := Hash(password)
	if err != nil {
		return err
	}
	return traducirEstado(s.repo.asignarContrasena(ctx, actorID, userID, hash))
}

// CambiarContrasena es la de la propia cuenta, con la actual. Devuelve una
// sesion nueva: el repositorio revoca todas, y la que pidio el cambio tiene que
// seguir viva.
func (s *Service) CambiarContrasena(ctx context.Context, userID, actual, nueva, ip, userAgent string) (Sesion, error) {
	u, err := s.repo.porID(ctx, userID)
	if err != nil {
		return Sesion{}, traducirEstado(err)
	}

	_, hash, err := s.repo.paraAutenticar(ctx, u.Email)
	if err != nil {
		return Sesion{}, traducirEstado(err)
	}
	coincide, err := Verify(actual, hash)
	if err != nil {
		return Sesion{}, err
	}
	if !coincide {
		// 400 y no 401: la sesion es buena, lo que esta mal es un campo, y el
		// formulario tiene que saber cual. Un 401 ademas haria que el cliente
		// intentara renovar la sesion.
		return Sesion{}, httpx.BadRequest("La contrasena actual no es correcta", httpx.FieldIssue{
			Field: "currentPassword", Code: "mismatch", Message: "No es tu contrasena actual",
		})
	}

	if prob := validarPassword(nueva); prob != nil {
		prob.Errors[0].Field = "newPassword"
		return Sesion{}, prob
	}
	if nueva == actual {
		return Sesion{}, httpx.BadRequest("La contrasena nueva es igual a la actual", httpx.FieldIssue{
			Field: "newPassword", Code: "same", Message: "Tiene que ser distinta de la actual",
		})
	}

	nuevoHash, err := Hash(nueva)
	if err != nil {
		return Sesion{}, err
	}
	if err := s.repo.cambiarContrasena(ctx, userID, nuevoHash); err != nil {
		return Sesion{}, traducirEstado(err)
	}

	u.MustChangePassword = false
	return s.emitir(ctx, u, ip, userAgent)
}

func (s *Service) Solicitudes(ctx context.Context, q url.Values) (paging.Page[SolicitudPendiente], error) {
	p, prob := listadoDeSolicitudes.Parse(q)
	if prob != nil {
		return paging.Page[SolicitudPendiente]{}, prob
	}
	items, total, err := s.repo.solicitudes(ctx, p)
	if err != nil {
		return paging.Page[SolicitudPendiente]{}, err
	}
	return paging.NewPage(items, p, total), nil
}
