// Package config traduce el entorno a una estructura, y falla ruidosamente
// cuando falta algo que no puede tener default.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env            string
	HTTPAddr       string
	DatabaseURL    string
	JWTSigningKey  string
	CookieSecure   bool
	APIDocsEnabled bool
	MigrateOnStart bool
}

func (c Config) IsDev() bool { return c.Env == "dev" }

// Load lee el entorno. Devuelve error nombrando TODAS las variables que faltan,
// no solo la primera: descubrir de una en una que faltan tres secretos son tres
// arranques fallidos.
//
// Un secreto con valor por omision es un secreto en produccion. Por eso
// DATABASE_URL y JWT_SIGNING_KEY no lo tienen, y el proceso no arranca sin
// ellas. Ver docs/08-infra-local.md.
func Load() (Config, error) {
	var missing []string

	require := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		Env:            withDefault("APP_ENV", "dev"),
		HTTPAddr:       withDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:    require("DATABASE_URL"),
		JWTSigningKey:  require("JWT_SIGNING_KEY"),
		CookieSecure:   boolWithDefault("COOKIE_SECURE", true),
		APIDocsEnabled: boolWithDefault("API_DOCS_ENABLED", false),
		// Migrar al arrancar es comodo en desarrollo y una carrera en
		// produccion: dos replicas aplicando la misma migracion a la vez. Por
		// eso el default es false y en produccion se usa `cmd/migrate` como
		// paso explicito del despliegue. Ver docs/08-infra-local.md.
		MigrateOnStart: boolWithDefault("MIGRATE_ON_START", false),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf(
			"faltan variables de entorno obligatorias: %s (no tienen valor por omision a proposito)",
			strings.Join(missing, ", "),
		)
	}

	// La UI de exploracion de la API no existe fuera de desarrollo, aunque
	// alguien deje la variable puesta por accidente al copiar un .env.
	if !cfg.IsDev() {
		cfg.APIDocsEnabled = false
	}

	return cfg, nil
}

func withDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// boolWithDefault cae al valor seguro cuando la variable trae basura, en vez de
// interpretarla. COOKIE_SECURE=si no debe significar false.
func boolWithDefault(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return v
}
