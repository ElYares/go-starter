package app

import (
	"io/fs"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Module es el contrato entre un modulo y el resto del sistema. Cuatro
// funciones, y nada mas: app no sabe que hay dentro de un modulo, solo lo monta.
//
// La prueba de que un modulo esta bien hecho es que borrar su carpeta y su
// linea de modules.go deja el proyecto compilando. Ver docs/02-modulos.md.
//
// Los modulos NO importan este paquete: en Go las interfaces son estructurales,
// asi que basta con que los metodos coincidan. Si tuvieran que importarlo,
// habria un ciclo con el registro.
type Module interface {
	// Name identifica al modulo en logs, metricas y en su tabla de migraciones.
	Name() string

	// Permissions son los permisos que este modulo inventa. rbac los cataloga
	// al arrancar; ninguna migracion los inserta a mano, porque entonces
	// quedarian huerfanos el dia que se borre el modulo.
	Permissions() []rbac.Permission

	// Migrations son las suyas, embebidas y numeradas localmente. Devolver nil
	// es valido: no todo modulo tiene esquema.
	Migrations() fs.FS

	// Routes es el unico lugar donde este modulo aparece en el router.
	Routes(r *httpx.Router)
}
