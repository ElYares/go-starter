// Package identity son los usuarios, los roles y el catalogo de permisos.
//
// Es el unico modulo del que otro puede depender —la excepcion escrita en
// limites_test.go— porque todo lo demas necesita saber quien hace la peticion.
// Esa excepcion es el motivo de que aqui no haya nada mas que identidad: cada
// cosa que se meta en este paquete se vuelve importable desde cualquier sitio.
package identity

import (
	"context"
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	svc  *Service
	repo *Repo
}

// New recibe si el entorno es de desarrollo, no lo lee del entorno: config es
// el unico paquete que traduce variables, y un modulo que consulta os.Getenv se
// vuelve imposible de probar en los dos modos.
func New(pool *pgxpool.Pool, dev bool) *Module {
	repo := &Repo{pool: pool}
	return &Module{svc: &Service{repo: repo, dev: dev}, repo: repo}
}

// Service es lo que app inyecta en los demas modulos cuando necesitan resolver
// al actor de una peticion.
func (m *Module) Service() *Service { return m.svc }

func (m *Module) Name() string { return "identity" }

func (m *Module) Permissions() []rbac.Permission {
	// Los cuatro van marcados como sensibles: son los que reparten poder en vez
	// de usarlo. Quien pueda asignar roles puede darse a si mismo cualquier
	// permiso, asi que concederlos al rol `admin` haria de `admin` un
	// superadmin con otro nombre y la separacion no separaria nada.
	return []rbac.Permission{
		{Key: "identity.user.read", Desc: "Ver las cuentas y sus roles", Sensitive: true},
		{Key: "identity.user.write", Desc: "Crear cuentas, cambiar sus datos y deshabilitarlas", Sensitive: true},
		{Key: "identity.role.read", Desc: "Ver los roles y que permisos concede cada uno", Sensitive: true},
		{Key: "identity.role.assign", Desc: "Asignar y quitar roles a una cuenta", Sensitive: true},
	}
}

func (m *Module) Migrations() fs.FS {
	// El error solo puede darse si el //go:embed de arriba no coincide con la
	// carpeta, y eso lo detecta el compilador. Un panic aqui es un bug, no una
	// condicion de ejecucion.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("identity: migrations/ no esta embebido: " + err.Error())
	}
	return sub
}

// SembrarPermisos hace de este modulo el dueno de la tabla `permissions`.
//
// app lo descubre por la interfaz, no por el nombre del paquete: quien tenga
// este metodo recibe el catalogo entero al migrar. Asi un fork que reemplace
// identity por otra cosa sigue funcionando, y uno que la borre del todo
// simplemente no siembra nada.
//
// Los permisos NO los declara este modulo: los declaran todos, y aqui solo se
// guardan. Ver docs/02-modulos.md.
func (m *Module) SembrarPermisos(ctx context.Context, perms []rbac.Permission) error {
	return m.repo.SembrarPermisos(ctx, perms)
}

// Routes no monta nada todavia, y es deliberado.
//
// HU-008 trae el esquema y las reglas; el login, el refresh y el `me` son de
// CU-001 a CU-003, y el CRUD de cuentas del dashboard es de una fase posterior.
// Un endpoint adelantado aqui seria superficie publica sin contrato ni prueba
// de autorizacion, que es peor que no tenerlo.
//
// El metodo existe igual porque es parte del contrato de app.Module: un modulo
// sin rutas es valido, uno que no cumple la interfaz no compila.
func (m *Module) Routes(*httpx.Router) {}

// SembrarSuperadminDeDesarrollo es lo que llama `cmd/seed`. La firma habla de
// cadenas y no de tipos de este paquete a proposito: es lo que permite que app
// la declare como interfaz sin importar el modulo.
func (m *Module) SembrarSuperadminDeDesarrollo(ctx context.Context, email, password, nombre string) error {
	_, err := m.svc.SembrarSuperadminDeDesarrollo(ctx, UsuarioNuevo{
		Email:       email,
		Password:    password,
		DisplayName: nombre,
		Roles:       []string{RolSuperadmin},
	})
	return err
}
