package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	_ "image/jpeg" // registran su formato en image.DecodeConfig
	_ "image/png"
	"io"
	"net/http"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/elyares/go-starter/api/internal/platform/storage"
)

// repositorio es la costura entre el service y Postgres, para que las reglas
// —el tipo, el tope, la dedup— se prueben sin base.
type repositorio interface {
	obtener(ctx context.Context, id uuid.UUID) (registro, error)
	porHash(ctx context.Context, sum []byte) (registro, error)
	insertar(ctx context.Context, n nuevo) (registro, bool, error)
}

type Service struct {
	repo  repositorio
	store storage.Store
}

// Subir guarda src y devuelve el registro, y si es nuevo. Lee src una sola vez,
// en streaming: el tipo sale de los primeros bytes, el hash se calcula mientras
// se copia, y el tope corta la copia en cuanto se pasa.
//
// Nada queda con nombre hasta que todo cuadra. Un archivo demasiado grande, de
// otro tipo o roto se descarta de lo provisional, y la fila se escribe al
// final: nunca hay una fila apuntando a un archivo que no existe.
func (s *Service) Subir(ctx context.Context, nombre string, src io.Reader) (Medio, bool, error) {
	limitado := &conTope{r: src, quedan: MaxBytes}

	inicio := make([]byte, cabecera)
	n, err := io.ReadFull(limitado, inicio)
	switch {
	case errors.Is(err, errDemasiadoGrande):
		return Medio{}, false, demasiadoGrande()
	case n == 0 && (err == nil || errors.Is(err, io.EOF)):
		return Medio{}, false, archivoVacio()
	case err != nil && !errors.Is(err, io.ErrUnexpectedEOF):
		return Medio{}, false, err
	}
	inicio = inicio[:n]

	mime := http.DetectContentType(inicio)
	formato, ok := permitidos[mime]
	if !ok {
		return Medio{}, false, tipoNoPermitido(mime)
	}

	hash := sha256.New()
	cuerpo := io.TeeReader(io.MultiReader(bytes.NewReader(inicio), limitado), hash)
	prov, err := s.store.Stage(ctx, cuerpo)
	if errors.Is(err, errDemasiadoGrande) {
		return Medio{}, false, demasiadoGrande()
	}
	if err != nil {
		return Medio{}, false, err
	}
	defer func() { _ = prov.Discard() }()

	ancho, alto, err := dimensiones(prov, formato)
	if err != nil {
		return Medio{}, false, err
	}

	sum := hash.Sum(nil)
	if existente, err := s.repo.porHash(ctx, sum); err == nil {
		return existente.Medio, false, nil
	} else if !errors.Is(err, errNoExiste) {
		return Medio{}, false, err
	}

	// Primero el archivo y despues la fila. Al reves, una fila podria quedar
	// apuntando a nada si el commit falla. Si dos subidas iguales llegan aqui a
	// la vez, las dos escriben la misma llave con los mismos bytes, y la base
	// decide cual fila queda.
	llave := llaveDe(sum)
	if err := prov.Commit(ctx, llave); err != nil {
		return Medio{}, false, err
	}

	reg, creado, err := s.repo.insertar(ctx, nuevo{
		sha256: sum,
		mime:   mime,
		tamano: int64(MaxBytes - limitado.quedan),
		ancho:  ancho,
		alto:   alto,
		nombre: nombreOriginal(nombre),
		llave:  llave,
	})
	if err != nil {
		return Medio{}, false, err
	}
	return reg.Medio, creado, nil
}

// dimensiones relee lo provisional. Que DetectContentType diga "image/png" solo
// significa que los primeros bytes lo parecen; DecodeConfig comprueba que la
// cabecera de la imagen se pueda leer, y que sea del formato que se detecto.
func dimensiones(prov storage.Staged, formato string) (int, int, error) {
	f, err := prov.Open()
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, leido, err := image.DecodeConfig(f)
	if err != nil || leido != formato || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, imagenRota()
	}
	return cfg.Width, cfg.Height, nil
}

func (s *Service) Leer(ctx context.Context, id uuid.UUID) (Medio, error) {
	reg, err := s.repo.obtener(ctx, id)
	if err != nil {
		return Medio{}, err
	}
	return reg.Medio, nil
}

// Abrir devuelve el registro y sus bytes. Quien llama cierra el lector.
func (s *Service) Abrir(ctx context.Context, id uuid.UUID) (Medio, io.ReadCloser, error) {
	reg, err := s.repo.obtener(ctx, id)
	if err != nil {
		return Medio{}, nil, err
	}
	rc, err := s.store.Open(ctx, reg.llave)
	if err != nil {
		// La fila existe y el archivo no: no es un 404, es algo que se rompio
		// del lado del servidor y alguien tiene que ver en el log.
		return Medio{}, nil, err
	}
	return reg.Medio, rc, nil
}

// conTope deja leer hasta `quedan` bytes y falla con errDemasiadoGrande en
// cuanto llega uno mas. No trunca en silencio: un archivo recortado a 5 MB
// seria una imagen rota guardada como si estuviera bien.
type conTope struct {
	r      io.Reader
	quedan int64
}

func (c *conTope) Read(p []byte) (int, error) {
	if c.quedan < 0 {
		return 0, errDemasiadoGrande
	}
	// Se pide uno mas de lo que queda para saber si hay mas.
	if int64(len(p)) > c.quedan+1 {
		p = p[:c.quedan+1]
	}
	n, err := c.r.Read(p)
	c.quedan -= int64(n)
	if c.quedan < 0 {
		return n + int(c.quedan), errDemasiadoGrande
	}
	return n, err
}
