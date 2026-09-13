package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// ResolverActor traduce el `sub` del token a los permisos efectivos de esa
// persona.
//
// Es una funcion y no una interfaz de un modulo porque plataforma no importa
// modulos: quien la implementa es `identity.Service.Actor`, y quien las une es
// `app`. Ver la Decision 001.
type ResolverActor func(ctx context.Context, userID string) (rbac.Actor, error)

// Sesion pone al actor en el contexto si la cookie `at` es valida, y no hace
// nada si no lo es.
//
// **No responde 401 nunca**, y eso es deliberado: quien decide si una ruta
// necesita sesion es el guard de la ruta, que sabe que permiso pide. Si este
// middleware rechazara, cerraria tambien la landing publica y el propio login.
//
// Un token invalido y la ausencia de token acaban igual —anonimo— porque para
// el que pide son lo mismo: no hay sesion. La diferencia se ve en el log.
func Sesion(f *Firmante, resolver ResolverActor, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieAT)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := f.Verificar(cookie.Value)
			if err != nil {
				// A nivel debug: un `at` vencido es lo normal cada quince
				// minutos, y registrarlo como aviso llenaria el log de ruido en
				// el unico caso que no es un problema.
				log.Debug("cookie de sesion descartada",
					slog.String("motivo", err.Error()))
				next.ServeHTTP(w, r)
				return
			}

			actor, err := resolver(r.Context(), claims.Sub)
			if err != nil {
				// Aqui si: el token es nuestro y esta vigente, pero no se pudo
				// resolver a quien nombra. O la base no contesta, o el usuario
				// se borro con la sesion abierta. Sigue como anonimo —el guard
				// dara 401— pero queda en el log, porque un 401 con cookie
				// valida es de las cosas que se ven como "me expulsa solo".
				log.Warn("no se pudo resolver el actor de una sesion valida",
					slog.String("sub", claims.Sub),
					slog.String("error", err.Error()))
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(rbac.WithActor(r.Context(), actor)))
		})
	}
}
