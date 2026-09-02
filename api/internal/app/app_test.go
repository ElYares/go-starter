package app

import (
	"io"
	"log/slog"
)

func loggerDePrueba() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
