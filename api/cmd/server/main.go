// Comando del servidor HTTP. Lo unico que hace: leer entorno, construir la
// aplicacion, servir, y apagarse sin cortar peticiones a medias.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elyares/go-starter/api/internal/app"
	"github.com/elyares/go-starter/api/internal/platform/config"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Todavia no hay logger estructurado: si la configuracion esta rota,
		// el mensaje tiene que ser legible en la consola de quien arranca.
		_, _ = os.Stderr.WriteString("configuracion invalida: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := observ.NewLogger(cfg.Env)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("no se pudo construir la aplicacion", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer a.Close()

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: a.Handler(),
		// Timeouts explicitos: los de omision de net/http son cero, es decir,
		// sin limite. Una conexion lenta que nunca termina de mandar cabeceras
		// se queda con un goroutine para siempre.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("servidor escuchando",
			slog.String("addr", cfg.HTTPAddr),
			slog.String("env", cfg.Env),
			slog.Bool("apiDocs", cfg.APIDocsEnabled),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Error("el servidor murio", slog.String("error", err.Error()))
		os.Exit(1)
	case <-ctx.Done():
		log.Info("apagando")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("apagado sucio", slog.String("error", err.Error()))
	}
}
