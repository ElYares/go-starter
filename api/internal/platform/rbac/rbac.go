// Package rbac contesta dos preguntas distintas, y hay que contestar las dos:
// si puedes hacer la operacion (permiso) y si el recurso es tuyo (propiedad).
//
// Aqui vive la primera. La segunda no es un middleware: se filtra EN LA
// CONSULTA, porque un `if` posterior se olvida y un metodo de repositorio que
// no compila sin el actor, no. Ver docs/04-reglas-de-crud.md seccion 5.
package rbac

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Permission la declara el modulo que la inventa, no una migracion. Sembrarla
// por SQL deja permisos huerfanos el dia que se borra el modulo.
type Permission struct {
	Key  string
	Desc string
}

// Actor es quien hace la peticion. En la fase 1 solo lo inyectan las pruebas;
// en la fase 2 lo llena el middleware de sesion a partir de la cookie.
type Actor struct {
	ID          string
	Permissions []string
}

func (a Actor) Can(key string) bool { return slices.Contains(a.Permissions, key) }

type ctxKey struct{}

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// ActorFrom devuelve el actor y si habia uno. El segundo valor importa: sin el,
// un actor vacio y "no hay sesion" serian indistinguibles, y eso convierte un
// 401 en un 403 o al reves.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(Actor)
	return a, ok
}

// Require exige un permiso con nombre.
//
// Cierra por omision en los dos sentidos: sin actor responde 401, y con actor
// sin el permiso responde 403. Una ruta que se registre con Require queda
// protegida sin que nadie se acuerde de protegerla.
//
// Ademas deja anotado el permiso en la ruta, que es lo que permite al arranque
// negarse a levantar si ningun modulo lo declaro.
func Require(key string) httpx.RouteOption {
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFrom(r.Context())
			if !ok {
				httpx.WriteProblem(w, r, httpx.Unauthorized())
				return
			}
			if !actor.Can(key) {
				httpx.WriteProblem(w, r, httpx.Forbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	return httpx.Combine(httpx.WithPermission(key), httpx.With(guard))
}

// Registry es el catalogo de permisos que declararon los modulos.
type Registry struct {
	byKey map[string]Permission
}

func NewRegistry(perms []Permission) (*Registry, error) {
	byKey := make(map[string]Permission, len(perms))
	for _, p := range perms {
		if p.Key == "" {
			return nil, fmt.Errorf("rbac: hay un permiso sin clave")
		}
		if _, repetido := byKey[p.Key]; repetido {
			// Dos modulos peleando por la misma clave es ambiguo, y el que gane
			// dependeria del orden del registro. Mejor no arrancar.
			return nil, fmt.Errorf("rbac: el permiso %q esta declarado dos veces", p.Key)
		}
		byKey[p.Key] = p
	}
	return &Registry{byKey: byKey}, nil
}

func (r *Registry) Has(key string) bool {
	_, ok := r.byKey[key]
	return ok
}

func (r *Registry) All() []Permission {
	out := make([]Permission, 0, len(r.byKey))
	for _, p := range r.byKey {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b Permission) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return out
}

// VerifyRoutes se corre al arrancar. Una ruta que exige un permiso que nadie
// declaro es un error de programacion que en produccion se veria como un 403
// permanente e inexplicable: el guard rechazaria a todos, incluido el admin.
// Mejor no levantar.
func VerifyRoutes(routes []httpx.Route, reg *Registry) error {
	for _, rt := range routes {
		if rt.Permission == "" {
			continue
		}
		if !reg.Has(rt.Permission) {
			return fmt.Errorf(
				"la ruta %s %s exige el permiso %q, que ningun modulo declara en Permissions()",
				rt.Method, rt.Pattern, rt.Permission)
		}
	}
	return nil
}
