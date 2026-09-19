package app

import (
	"io"
	"log/slog"
	"testing"
)

func loggerDePrueba() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// appDePrueba monta la aplicacion sin modulos y sin base: alcanza para las
// rutas de plataforma, y deja claro cuales de ellas NO dependen de Postgres.
func appDePrueba(t *testing.T, mods ...Module) *App {
	t.Helper()
	a := &App{log: loggerDePrueba()}
	// La misma cadena que produccion, no una recortada: si las pruebas armaran
	// un Handler sin CSRF ni sesion, ejercitarian un servidor que no existe.
	cfg := configDePrueba()
	if err := a.armarSesion(cfg.JWTSigningKey, cfg.CookieSecure); err != nil {
		t.Fatalf("armarSesion: %v", err)
	}
	a.armarLimite(cfg.SSRSecret)
	if err := a.montar(mods); err != nil {
		t.Fatalf("montar: %v", err)
	}
	return a
}
