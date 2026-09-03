// Comando de siembra: deja la base lista para trabajar en desarrollo.
//
// Aplica las migraciones, reconcilia el catalogo de permisos y crea el
// superadmin con el que se entra al dashboard. Se puede correr las veces que
// haga falta: las tres cosas son idempotentes.
//
// SOLO CORRE CON APP_ENV=dev. Un seed que se pueda ejecutar en produccion es
// una cuenta con contrasena conocida esperando a que alguien lo lance por
// costumbre, y esa cuenta no se nota hasta que la usan.
package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/elyares/go-starter/api/internal/app"
	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/db"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// Los valores por omision son publicos y estan bien que lo sean: solo existen
// en desarrollo, y una contrasena que hay que buscar en un chat para levantar
// el proyecto acaba pegada en el README de todas formas. Se cambian con las
// variables de entorno de al lado.
const (
	emailPorOmision    = "superadmin@go-starter.localhost"
	passwordPorOmision = "superadmin-de-desarrollo"
	nombrePorOmision   = "Superadmin de desarrollo"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		salir("configuracion invalida: " + err.Error())
	}

	log := observ.NewLogger(cfg.Env)

	if !cfg.IsDev() {
		// Antes de abrir la conexion siquiera. Este es el cinturon que se ve
		// desde fuera; el service tiene el suyo, para el dia que alguien llame
		// a la siembra desde otro sitio que no pase por aqui.
		salir("cmd/seed solo corre con APP_ENV=dev, y APP_ENV es " + cfg.Env)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("no se pudo abrir la conexion", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := app.RunMigrations(ctx, cfg, log, pool); err != nil {
		log.Error("fallo la migracion", "error", err)
		os.Exit(1)
	}

	// El orden no es casual: sin el catalogo sembrado, el rol superadmin se
	// asigna igual pero no concede nada, y el dashboard recibiria un 403 en
	// todo.
	if err := app.SeedPermissions(ctx, cfg, log, pool); err != nil {
		log.Error("fallo la siembra de permisos", "error", err)
		os.Exit(1)
	}

	email := conOmision("SEED_SUPERADMIN_EMAIL", emailPorOmision)
	password := conOmision("SEED_SUPERADMIN_PASSWORD", passwordPorOmision)
	nombre := conOmision("SEED_SUPERADMIN_NAME", nombrePorOmision)

	if err := app.SeedDevSuperadmin(ctx, cfg, log, pool, email, password, nombre); err != nil {
		log.Error("fallo la siembra del superadmin", "error", err)
		os.Exit(1)
	}

	// Por stdout y no por el log: es lo que la persona que acaba de correr el
	// comando necesita leer, y el log de desarrollo sale en JSON.
	_, _ = os.Stdout.WriteString("\nlisto. entra en /admin con:\n  " + email + "\n  " + password + "\n")
}

func conOmision(clave, omision string) string {
	if v := strings.TrimSpace(os.Getenv(clave)); v != "" {
		return v
	}
	return omision
}

func salir(mensaje string) {
	_, _ = os.Stderr.WriteString(mensaje + "\n")
	os.Exit(1)
}
