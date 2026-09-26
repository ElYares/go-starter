package app

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// moduloFalso implementa el contrato sin importar el paquete app: es
// exactamente lo que hacen los modulos de verdad, y comprobarlo aqui asegura
// que la interfaz no exija nada raro.
type moduloFalso struct {
	nombre  string
	permisT []rbac.Permission
	rutas   func(r *httpx.Router)
}

func (m moduloFalso) Name() string                   { return m.nombre }
func (m moduloFalso) Permissions() []rbac.Permission { return m.permisT }
func (m moduloFalso) Migrations() fs.FS              { return nil }
func (m moduloFalso) Routes(r *httpx.Router)         { m.rutas(r) }

func TestLasRutasDeUnModuloQuedanMontadas(t *testing.T) {
	mod := moduloFalso{
		nombre: "demo",
		rutas: func(r *httpx.Router) {
			r.Group("/api/v1", func(r *httpx.Router) {
				r.Get("/demo", func(w http.ResponseWriter, req *http.Request) {
					httpx.WriteJSON(w, req, http.StatusOK, map[string]string{"ok": "si"})
				})
			})
		},
	}

	a := appDePrueba(t, mod)

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/demo", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d; la ruta del modulo no quedo montada", rec.Code)
	}
}

// Este es el motivo de que el router guarde las rutas.
//
// Una clave mal escrita en Require() no falla al compilar. En produccion se
// veria como un 403 permanente e inexplicable, porque el guard rechazaria a
// todos, incluido el admin. Mejor no levantar.
func TestUnaRutaConPermisoNoDeclaradoImpideArrancar(t *testing.T) {
	mod := moduloFalso{
		nombre:  "demo",
		permisT: []rbac.Permission{{Key: "demo.read", Area: "Demo"}},
		rutas: func(r *httpx.Router) {
			// Con una errata: se declaro demo.read, no demo.raed.
			r.Get("/api/v1/demo", func(http.ResponseWriter, *http.Request) {}, rbac.Require("demo.raed"))
		},
	}

	a := &App{log: loggerDePrueba()}
	err := a.montar([]Module{mod})

	if err == nil {
		t.Fatal("montar tenia que fallar: la ruta exige un permiso que nadie declara")
	}
	if !strings.Contains(err.Error(), "demo.raed") {
		t.Errorf("el error tiene que nombrar el permiso culpable; dijo: %v", err)
	}
}

// Dos modulos peleando por la misma clave es ambiguo, y el que gane dependeria
// del orden del registro.
func TestDosModulosNoPuedenDeclararElMismoPermiso(t *testing.T) {
	sinRutas := func(*httpx.Router) {}
	a := &App{log: loggerDePrueba()}

	err := a.montar([]Module{
		moduloFalso{nombre: "uno", permisT: []rbac.Permission{{Key: "comun.read", Area: "Comun"}}, rutas: sinRutas},
		moduloFalso{nombre: "dos", permisT: []rbac.Permission{{Key: "comun.read", Area: "Comun"}}, rutas: sinRutas},
	})

	if err == nil {
		t.Fatal("montar tenia que fallar por el permiso duplicado")
	}
	if !strings.Contains(err.Error(), "comun.read") {
		t.Errorf("el error tiene que nombrar la clave repetida; dijo: %v", err)
	}
}

// El registro es explicito a proposito. Esta prueba falla si alguien agrega un
// modulo sin pensar, y obliga a que el orden —que es el de las migraciones— sea
// una decision visible.
// configDePrueba trae lo minimo para armar el registro. La llave de firma no
// puede ir vacia: `Modules` se niega a construir el modulo de identidad sin
// ella, y eso es a proposito —un starter que firma con una llave vacia emite
// tokens que cualquiera reproduce.
func configDePrueba() config.Config {
	return config.Config{
		Env:           "dev",
		JWTSigningKey: "llave-de-prueba-no-usar-fuera-de-aqui",
		StoragePath:   filepath.Join(os.TempDir(), "go-starter-pruebas-de-app"),
		SSRSecret:     "secreto-de-prueba-del-ssr",
	}
}

func modulosDePrueba(t *testing.T) []Module {
	t.Helper()
	mods, err := Modules(configDePrueba(), nil)
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}
	return mods
}

// Sin llave de firma no hay registro, y el error lo dice. Es la unica forma de
// que un despliegue mal configurado se note al arrancar y no la primera vez que
// alguien inicia sesion.
func TestSinLlaveDeFirmaElRegistroNoSeArma(t *testing.T) {
	if _, err := Modules(config.Config{Env: "dev"}, nil); err == nil {
		t.Fatal("se armo el registro con la llave de firma vacia")
	}
}

func TestElRegistroDeclaraLosModulosEsperados(t *testing.T) {
	nombres := []string{}
	for _, m := range modulosDePrueba(t) {
		nombres = append(nombres, m.Name())
	}

	// identity va primero porque es el orden de las migraciones, y las llaves
	// foraneas de los demas apuntan a sus usuarios. Cambiar este orden no es
	// cosmetico: rompe la migracion en una base vacia.
	esperado := []string{"identity", "settings", "media", "content"}
	if strings.Join(nombres, ",") != strings.Join(esperado, ",") {
		t.Errorf("modulos registrados = %v, se esperaba %v\nsi agregaste uno, actualiza esta prueba y confirma que su posicion en la lista es la que quieres: es el orden en que corren las migraciones", nombres, esperado)
	}
}

// Las TRES interfaces opcionales del registro no se comprueban al compilar: un
// type assertion que no encaja devuelve false y sigue. Si a `SembrarPermisos`,
// a `SembrarSuperadminDeDesarrollo` o a `Actor` se les cambia un parametro y el
// modulo no se entera, todo compila, el arranque no protesta, y lo unico que
// pasa es que la funcion se vuelve un no-op silencioso.
//
// Esta prueba es lo que convierte ese silencio en un fallo. Ya paso una vez:
// la interfaz decia `displayName` donde el modulo decia `nombre`.
//
// `Actor` es la que mas caro sale de perder: sin ella ninguna peticion tiene
// sesion, TODA ruta con guard responde 401, y el sintoma —"no puedo entrar a
// nada"— no se parece en nada a la causa.
func TestElRegistroTieneQuienSiembreElCatalogoElSuperadminYResuelvaActores(t *testing.T) {
	mods := modulosDePrueba(t)

	var catalogos, sembradores, resolvedores []string
	for _, m := range mods {
		if _, ok := m.(CatalogoDePermisos); ok {
			catalogos = append(catalogos, m.Name())
		}
		if _, ok := m.(SembradorDeSuperadmin); ok {
			sembradores = append(sembradores, m.Name())
		}
		if _, ok := m.(ResolverDeActores); ok {
			resolvedores = append(resolvedores, m.Name())
		}
	}

	if len(catalogos) != 1 {
		t.Errorf("modulos que guardan el catalogo de permisos = %v; tiene que haber exactamente uno", catalogos)
	}
	if len(sembradores) != 1 {
		t.Errorf("modulos que siembran el superadmin = %v; tiene que haber exactamente uno", sembradores)
	}
	if len(resolvedores) != 1 {
		t.Errorf("modulos que resuelven actores = %v; tiene que haber exactamente uno", resolvedores)
	}
}
