package identity

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Los handlers de la contrasena temporal (HU-019). Las firmas las dicta
// ServerInterface.

// PedirContrasena responde lo mismo exista o no la cuenta. Ver el service.
func (m *Module) PedirContrasena(w http.ResponseWriter, r *http.Request, _ PedirContrasenaParams) {
	var cuerpo PedidoDeContrasena
	if prob := httpx.DecodeJSON(w, r, &cuerpo); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}
	if err := m.svc.PedirContrasena(r.Context(), cuerpo.Email, httpx.ClientIP(r),
		recortar(r.UserAgent(), maxUserAgent)); err != nil {
		escribirError(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusAccepted, PedidoRecibido{Message: PedidoRecibidoMensaje})
}

// AsignarContrasena necesita al actor, no solo su permiso: la regla de poder
// compara sus permisos con los de la cuenta.
func (m *Module) AsignarContrasena(w http.ResponseWriter, r *http.Request, id CuentaId) {
	actor, ok := rbac.ActorFrom(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, httpx.Unauthorized())
		return
	}

	var cuerpo ContrasenaTemporal
	if prob := httpx.DecodeJSON(w, r, &cuerpo); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}
	if err := m.svc.AsignarContrasena(r.Context(), actor.ID, id.String(), cuerpo.Password); err != nil {
		escribirError(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// CambiarContrasena emite cookies nuevas: el service revoco todas las sesiones
// de la cuenta, incluida la que hizo el cambio, y le abrio otra.
func (m *Module) CambiarContrasena(w http.ResponseWriter, r *http.Request) {
	actor, ok := rbac.ActorFrom(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, httpx.Unauthorized())
		return
	}

	var cuerpo CambioDeContrasena
	if prob := httpx.DecodeJSON(w, r, &cuerpo); prob != nil {
		httpx.WriteProblem(w, r, prob)
		return
	}
	sesion, err := m.svc.CambiarContrasena(r.Context(), actor.ID, cuerpo.CurrentPassword, cuerpo.NewPassword,
		httpx.ClientIP(r), recortar(r.UserAgent(), maxUserAgent))
	if err != nil {
		escribirError(w, r, err)
		return
	}
	m.emisor.Emitir(w, sesion.AccessToken, sesion.RefreshToken, sesion.TokenCSRF)
	httpx.NoContent(w)
}

func (m *Module) ListarSolicitudes(w http.ResponseWriter, r *http.Request, _ ListarSolicitudesParams) {
	pagina, err := m.svc.Solicitudes(r.Context(), r.URL.Query())
	if err != nil {
		escribirError(w, r, err)
		return
	}

	solicitudes := make([]Solicitud, len(pagina.Content))
	for i, s := range pagina.Content {
		id, err1 := uuid.Parse(s.ID)
		userID, err2 := uuid.Parse(s.UserID)
		if err1 != nil || err2 != nil {
			escribirError(w, r, fmt.Errorf("identity: solicitud %q con ids que no son uuid", s.ID))
			return
		}
		solicitudes[i] = Solicitud{Id: id, UserId: userID, Email: s.Email, DisplayName: s.DisplayName, CreatedAt: s.CreatedAt}
	}
	httpx.WriteJSON(w, r, http.StatusOK, SolicitudesPage{Content: solicitudes, Page: PageMeta(pagina.Page)})
}
