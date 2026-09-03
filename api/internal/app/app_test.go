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
	if err := a.montar(mods); err != nil {
		t.Fatalf("montar: %v", err)
	}
	return a
}
