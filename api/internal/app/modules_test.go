package app

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		permisT: []rbac.Permission{{Key: "demo.read"}},
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
		moduloFalso{nombre: "uno", permisT: []rbac.Permission{{Key: "comun.read"}}, rutas: sinRutas},
		moduloFalso{nombre: "dos", permisT: []rbac.Permission{{Key: "comun.read"}}, rutas: sinRutas},
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
func TestElRegistroDeclaraLosModulosEsperados(t *testing.T) {
	nombres := []string{}
	for _, m := range Modules(nil) {
		nombres = append(nombres, m.Name())
	}

	esperado := []string{"settings"}
	if strings.Join(nombres, ",") != strings.Join(esperado, ",") {
		t.Errorf("modulos registrados = %v, se esperaba %v\nsi agregaste uno, actualiza esta prueba y confirma que su posicion en la lista es la que quieres: es el orden en que corren las migraciones", nombres, esperado)
	}
}
