package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/modules/content"
	"github.com/elyares/go-starter/api/internal/modules/identity"
	"github.com/elyares/go-starter/api/internal/modules/settings"
	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/db"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// modules es la tabla de contenidos del backend, escrita a mano.
//
// No hay descubrimiento automatico ni init() con efectos secundarios: la magia
// funciona hasta que alguien pregunta por que existe una ruta y no hay donde
// mirar.
//
// **El orden importa: es el orden en que corren las migraciones.** Un modulo
// solo puede tener llave foranea hacia otro que se registre antes que el, y en
// la practica eso significa hacia identity. Si dos se necesitan mutuamente, uno
// de los dos esta mal cortado.
// Devuelve error porque armar el modulo de identidad exige una llave de firma
// valida, y un starter que arranca con una llave vacia firma tokens que
// cualquiera puede reproducir. Es preferible no levantar. Por lo mismo falla si
// el catalogo de bloques de content no compila.
func Modules(cfg config.Config, pool *pgxpool.Pool) ([]Module, error) {
	firmante, err := auth.NewFirmante(cfg.JWTSigningKey)
	if err != nil {
		return nil, err
	}

	// content compila su catalogo de bloques al armarse: un esquema roto no
	// levanta, en vez de ser un 500 en el primer guardado.
	contenido, err := content.New(pool)
	if err != nil {
		return nil, err
	}

	return []Module{
		// primero: los demas dependen de el
		identity.New(pool, cfg.IsDev(), firmante, auth.NewIntentos(), auth.NewEmisor(cfg.CookieSecure)),
		settings.New(pool),
		contenido,
		// catalog.New(...),    // <- un fork agrega su dominio aqui
	}, nil
}

// CatalogoDePermisos lo implementa el modulo dueno de la tabla `permissions`.
//
// Se declara aqui, del lado del consumidor, y no como un quinto metodo de
// Module: la mayoria de los modulos declaran permisos y ninguno de ellos los
// guarda. Obligar a todos a implementar un metodo que solo uno usa convierte la
// interfaz en un formulario que se rellena con cuerpos vacios.
type CatalogoDePermisos interface {
	SembrarPermisos(ctx context.Context, perms []rbac.Permission) error
}

// RunMigrations aplica lo pendiente de todos los modulos. La usan dos sitios:
// `cmd/migrate` como paso explicito del despliegue, y el arranque del servidor
// cuando MIGRATE_ON_START esta puesto, que en la practica es solo desarrollo.
func RunMigrations(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	mods, err := Modules(cfg, pool)
	if err != nil {
		return err
	}
	migratables := make([]db.Migratable, 0, len(mods))
	for _, m := range mods {
		migratables = append(migratables, m)
	}
	return db.Migrate(ctx, cfg.DatabaseURL, log, migratables...)
}

// SeedPermissions vuelca en la base el catalogo que declararon los modulos.
//
// Corre pegada a las migraciones y no en cada arranque, por dos razones. Una:
// es un cambio de esquema logico, del mismo tipo que una migracion, y en
// produccion tiene que ser un paso explicito por lo mismo —dos replicas
// reconciliando a la vez es una carrera. Dos: `db.Open` no conecta, asi que el
// servidor levanta con la base caida y /readyz lo reporta; sembrar en cada
// arranque cambiaria eso por un proceso que no arranca y no puede explicar por
// que.
//
// Es idempotente: alta, actualizacion y borrado de lo que ya nadie declara.
// Volver a correrla no duplica nada.
func SeedPermissions(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	mods, err := Modules(cfg, pool)
	if err != nil {
		return err
	}

	reg, err := rbac.NewRegistry(permisosDe(mods))
	if err != nil {
		return err
	}

	var dueno Module
	for _, m := range mods {
		if _, ok := m.(CatalogoDePermisos); !ok {
			continue
		}
		if dueno != nil {
			// Dos modulos guardando el mismo catalogo es ambiguo: el que gane
			// dependeria del orden del registro, y el otro tendria una tabla
			// que nadie mantiene. Mejor no sembrar.
			return fmt.Errorf(
				"los modulos %s y %s dicen ser dueños del catalogo de permisos", dueno.Name(), m.Name())
		}
		dueno = m
	}

	if dueno == nil {
		// Un fork puede haber borrado identity entero. Sin tabla donde
		// guardarlos, los permisos siguen valiendo en memoria y los guards
		// siguen funcionando: no hay nada que sembrar y no es un error.
		log.Debug("ningun modulo guarda el catalogo de permisos: no se siembra nada")
		return nil
	}

	if err := dueno.(CatalogoDePermisos).SembrarPermisos(ctx, reg.All()); err != nil {
		return fmt.Errorf("sembrando el catalogo de permisos en %s: %w", dueno.Name(), err)
	}

	log.Info("catalogo de permisos al dia",
		slog.String("modulo", dueno.Name()), slog.Int("permisos", len(reg.All())))
	return nil
}

// permisosDe junta lo que declara cada modulo. Lo usan el montaje del router y
// la siembra, y esta escrito una vez para que no puedan discrepar: un catalogo
// en la base distinto del que verifica las rutas seria un 403 sin explicacion.
func permisosDe(mods []Module) []rbac.Permission {
	var perms []rbac.Permission
	for _, m := range mods {
		perms = append(perms, m.Permissions()...)
	}
	return perms
}

// ResolverDeActores lo implementa el modulo que sabe que puede hacer cada
// usuario. Es la costura por la que el middleware de sesion —que es plataforma,
// y por tanto no puede importar identity— llega a los permisos.
//
// Se declara aqui por la misma razon que las otras dos: solo un modulo la
// implementa, y un fork que borre identity tiene que seguir compilando. Lo que
// pasa entonces es que nadie resuelve actores, ninguna sesion se llena y todas
// las rutas con guard responden 401 — que es lo correcto para una instalacion
// sin modulo de identidad.
type ResolverDeActores interface {
	Actor(ctx context.Context, userID string) (rbac.Actor, error)
}

// SembradorDeSuperadmin lo implementa el modulo que sabe crear una cuenta.
// Misma razon que CatalogoDePermisos para declararlo aqui y no en Module: solo
// uno lo implementa, y un fork que borre identity tiene que seguir compilando.
type SembradorDeSuperadmin interface {
	SembrarSuperadminDeDesarrollo(ctx context.Context, email, password, displayName string) error
}

// SeedDevSuperadmin crea la cuenta con la que se entra al dashboard en
// desarrollo. Es superadmin y no admin a proposito: en desarrollo hace falta
// poder tocarlo todo, incluidos los roles y los permisos.
//
// Vive en app y no en cmd/seed para que `cmd/` no importe ningun modulo: si lo
// hiciera, borrar identity dejaria de ser "una carpeta y una linea del
// registro" y romperia la compilacion de un binario que ni lo nombra.
func SeedDevSuperadmin(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, email, password, nombre string) error {
	mods, err := Modules(cfg, pool)
	if err != nil {
		return err
	}

	for _, m := range mods {
		sembrador, ok := m.(SembradorDeSuperadmin)
		if !ok {
			continue
		}
		if err := sembrador.SembrarSuperadminDeDesarrollo(ctx, email, password, nombre); err != nil {
			return fmt.Errorf("sembrando el superadmin en %s: %w", m.Name(), err)
		}
		log.Info("superadmin de desarrollo listo",
			slog.String("modulo", m.Name()), slog.String("email", email))
		return nil
	}

	log.Warn("ningun modulo sabe crear cuentas: no hay superadmin que sembrar")
	return nil
}
