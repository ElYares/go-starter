package httpx

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type Middleware func(http.Handler) http.Handler

// Route es lo que quedo registrado. Se guarda para poder revisarlo al arrancar:
// es lo que permite que la aplicacion se niegue a levantar si una ruta exige un
// permiso que ningun modulo declaro.
type Route struct {
	Method     string
	Pattern    string
	Permission string // vacio = ruta publica, y eso tiene que ser deliberado
}

// RouteOption configura una ruta al registrarla. `rbac.Require` devuelve una.
type RouteOption func(*routeConfig)

type routeConfig struct {
	permission string
	mws        []Middleware
}

// WithPermission marca la ruta como protegida. httpx solo guarda la cadena; que
// signifique algo es cosa de rbac. Asi httpx no depende de un modelo de
// permisos concreto y un fork puede cambiarlo sin tocar el router.
func WithPermission(key string) RouteOption {
	return func(c *routeConfig) { c.permission = key }
}

// With agrega middleware solo a esta ruta.
func With(mw ...Middleware) RouteOption {
	return func(c *routeConfig) { c.mws = append(c.mws, mw...) }
}

// Combine junta varias opciones en una. Existe para que otro paquete —rbac—
// pueda ofrecer una opcion propia sin que httpx tenga que conocerlo.
func Combine(opts ...RouteOption) RouteOption {
	return func(c *routeConfig) {
		for _, o := range opts {
			o(c)
		}
	}
}

// Router envuelve http.ServeMux para dar tres cosas que el estandar no tiene:
// prefijos de grupo, middleware por grupo y por ruta, y el registro de lo que
// se monto.
//
// No es un framework. Si algun dia estorba, quitarlo es reescribir estas cien
// lineas, no migrar el proyecto.
type Router struct {
	mux    *http.ServeMux
	prefix string
	mws    []Middleware
	routes *[]Route
}

// NewRouter deja montado el comodin que contesta problem+json a lo que ningun
// patron reclame. Sin el, el 404 de omision de net/http sale en texto plano y
// rompe la forma unica justo en el error mas frecuente.
func NewRouter() *Router {
	r := &Router{mux: http.NewServeMux(), routes: &[]Route{}}
	r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		WriteProblem(w, req, NotFound())
	})
	return r
}

// Group abre un subrouter con prefijo.
//
// El middleware del padre se copia. Hoy no es observable desde fuera —la cadena
// de cada ruta se materializa al registrarla, asi que un grupo posterior ya no
// puede afectar a lo construido antes— pero el clon cuesta nada y deja de ser
// gratis el dia que alguien retenga el *Router hijo fuera del closure.
// Se documenta asi a proposito: no hay una prueba que lo respalde porque no se
// puede escribir una que falle sin el.
func (r *Router) Group(prefix string, fn func(*Router)) {
	fn(&Router{
		mux:    r.mux,
		prefix: r.prefix + prefix,
		mws:    slices.Clone(r.mws),
		routes: r.routes,
	})
}

// Use agrega middleware a las rutas que se registren DESPUES, en este router.
func (r *Router) Use(mw ...Middleware) { r.mws = append(r.mws, mw...) }

func (r *Router) Get(p string, h http.HandlerFunc, o ...RouteOption) {
	r.handle(http.MethodGet, p, h, o...)
}
func (r *Router) Post(p string, h http.HandlerFunc, o ...RouteOption) {
	r.handle(http.MethodPost, p, h, o...)
}
func (r *Router) Put(p string, h http.HandlerFunc, o ...RouteOption) {
	r.handle(http.MethodPut, p, h, o...)
}
func (r *Router) Patch(p string, h http.HandlerFunc, o ...RouteOption) {
	r.handle(http.MethodPatch, p, h, o...)
}
func (r *Router) Delete(p string, h http.HandlerFunc, o ...RouteOption) {
	r.handle(http.MethodDelete, p, h, o...)
}

func (r *Router) handle(method, pattern string, h http.HandlerFunc, opts ...RouteOption) {
	cfg := &routeConfig{}
	for _, o := range opts {
		o(cfg)
	}

	full := r.prefix + pattern
	if !strings.HasPrefix(full, "/") {
		// Un patron sin barra inicial hace que ServeMux entre en panico con un
		// mensaje que no dice de que modulo salio.
		panic(fmt.Sprintf("httpx: la ruta %q %q no empieza con /", method, full))
	}

	*r.routes = append(*r.routes, Route{Method: method, Pattern: full, Permission: cfg.permission})

	// Primero el del grupo, despues el de la ruta: lo mas general envuelve a lo
	// mas especifico, igual que en la cadena global.
	r.mux.Handle(method+" "+full, chain(h, append(slices.Clone(r.mws), cfg.mws...)...))
}

// Routes devuelve lo registrado, para revisarlo al arrancar.
func (r *Router) Routes() []Route { return slices.Clone(*r.routes) }

func (r *Router) Handler() http.Handler { return r.mux }

func chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
