package app_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Las reglas de dependencia de docs/01-arquitectura.md se verifican, no se
// confian a la disciplina. Es lo que impide que la separacion se erosione en el
// mes tres, cuando alguien "solo necesita una funcion de alla".
//
// Se leen los imports con go/parser en vez de x/tools/go/packages: no hace
// falta resolver tipos para saber que importa cada archivo, y evita arrastrar
// una dependencia grande a las pruebas.

const modulo = "github.com/elyares/go-starter/api/"

// importsPorPaquete devuelve, por cada directorio bajo internal/, sus imports
// del propio modulo. Incluye los archivos _test.go a proposito: una prueba que
// cruza la frontera tambien la rompe.
func importsPorPaquete(t *testing.T) map[string][]string {
	t.Helper()

	raiz, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	out := map[string][]string{}
	fset := token.NewFileSet()

	err = filepath.WalkDir(filepath.Join(raiz, "internal"), func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		archivo, err := parser.ParseFile(fset, ruta, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(raiz, filepath.Dir(ruta))
		if err != nil {
			return err
		}
		paquete := filepath.ToSlash(rel)

		for _, imp := range archivo.Imports {
			ruta := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(ruta, modulo) {
				out[paquete] = append(out[paquete], strings.TrimPrefix(ruta, modulo))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(out) == 0 {
		// Sin esto, un WalkDir que no encuentra nada haria pasar todas las
		// reglas por vacuidad, y el reporte diria verde sin haber mirado nada.
		t.Fatal("no se leyo ningun paquete: la prueba estaria pasando por vacuidad")
	}
	return out
}

// Regla 1: la plataforma no conoce a los modulos. Si necesita algo de uno, ese
// algo va como interfaz que el modulo implementa (ver db.Migratable).
func TestPlataformaNoImportaModulos(t *testing.T) {
	for paquete, imports := range importsPorPaquete(t) {
		if !strings.HasPrefix(paquete, "internal/platform/") {
			continue
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, "internal/modules/") {
				t.Errorf("%s importa %s\nla plataforma no puede depender de un modulo: usa una interfaz declarada del lado del consumidor", paquete, imp)
			}
			if strings.HasPrefix(imp, "internal/app") {
				t.Errorf("%s importa %s\nla plataforma no puede depender de app: seria un import hacia arriba", paquete, imp)
			}
		}
	}
}

// Regla 2: un modulo no importa otro modulo. Si necesita datos ajenos, los pide
// por una interfaz de su propio ports.go y app inyecta la implementacion.
//
// La excepcion documentada es identity: cualquiera puede depender de su Actor.
func TestUnModuloNoImportaOtroModulo(t *testing.T) {
	const excepcion = "internal/modules/identity"

	for paquete, imports := range importsPorPaquete(t) {
		if !strings.HasPrefix(paquete, "internal/modules/") {
			continue
		}
		propio := paqueteDelModulo(paquete)

		for _, imp := range imports {
			if !strings.HasPrefix(imp, "internal/modules/") {
				continue
			}
			if paqueteDelModulo(imp) == propio || strings.HasPrefix(imp, excepcion) {
				continue
			}
			t.Errorf("%s importa %s\nun modulo no puede importar otro: declara una interfaz en su ports.go y deja que app la conecte", paquete, imp)
		}
	}
}

// Regla 3: solo app conoce el grafo completo. Si otro paquete importa un
// modulo, borrar ese modulo deja de ser "una carpeta y una linea".
func TestSoloAppConoceLosModulos(t *testing.T) {
	for paquete, imports := range importsPorPaquete(t) {
		if paquete == "internal/app" || strings.HasPrefix(paquete, "internal/modules/") {
			continue
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, "internal/modules/") {
				t.Errorf("%s importa %s\nsolo internal/app puede conocer los modulos", paquete, imp)
			}
		}
	}
}

// El molde tiene que seguir compilando. Vive bajo _template, que `go build ./...`
// ignora por el guion bajo, asi que sin esta prueba se pudriria en silencio y el
// primer fork lo descubriria copiando codigo roto.
func TestElModuloPlantillaCompila(t *testing.T) {
	cmd := exec.Command("go", "build", "./internal/modules/_template/...")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("el molde de modulo no compila:\n%s", out)
	}
}

func paqueteDelModulo(ruta string) string {
	partes := strings.Split(ruta, "/")
	if len(partes) < 3 {
		return ruta
	}
	return strings.Join(partes[:3], "/")
}
