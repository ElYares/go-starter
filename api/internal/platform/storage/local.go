package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// llaveValida admite segmentos de minusculas, digitos, punto, guion y guion
// bajo separados por `/`. Sin `..`, sin ruta absoluta, sin barra invertida: una
// llave no puede salirse de la raiz aunque alguien la arme mal.
var llaveValida = regexp.MustCompile(`^[a-z0-9_-][a-z0-9._-]*(/[a-z0-9_-][a-z0-9._-]*)*$`)

// Local guarda en un directorio del disco. Es la implementacion de desarrollo,
// y sirve en produccion con un volumen persistente y una sola replica.
type Local struct {
	raiz string
}

// NewLocal crea la raiz si no existe y comprueba que se puede escribir en ella.
// Un directorio sin permisos tiene que impedir arrancar, no ser un 500 en la
// primera subida.
func NewLocal(raiz string) (*Local, error) {
	if raiz == "" {
		return nil, errors.New("storage: la raiz esta vacia")
	}
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, tmpDir), 0o750); err != nil {
		return nil, fmt.Errorf("storage: no se pudo crear %s: %w", abs, err)
	}
	prueba, err := os.CreateTemp(filepath.Join(abs, tmpDir), "prueba-*")
	if err != nil {
		return nil, fmt.Errorf("storage: no se puede escribir en %s: %w", abs, err)
	}
	_ = prueba.Close()
	_ = os.Remove(prueba.Name())
	return &Local{raiz: abs}, nil
}

// tmpDir vive DENTRO de la raiz a proposito: os.Rename solo es atomico en el
// mismo sistema de archivos, y el /tmp del sistema suele ser otro.
const tmpDir = ".tmp"

func (l *Local) Stage(ctx context.Context, src io.Reader) (Staged, error) {
	f, err := os.CreateTemp(filepath.Join(l.raiz, tmpDir), "subida-*")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, lectorConContexto{ctx: ctx, r: src})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return nil, err
	}
	return &staged{l: l, ruta: f.Name()}, nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	ruta, err := l.ruta(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) ruta(key string) (string, error) {
	if !llaveValida.MatchString(key) {
		return "", ErrInvalidKey
	}
	return filepath.Join(l.raiz, filepath.FromSlash(key)), nil
}

type staged struct {
	l     *Local
	ruta  string
	hecho bool
}

func (s *staged) Open() (io.ReadCloser, error) { return os.Open(s.ruta) }

func (s *staged) Commit(_ context.Context, key string) error {
	destino, err := s.l.ruta(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o750); err != nil {
		return err
	}
	if err := os.Rename(s.ruta, destino); err != nil {
		return err
	}
	s.hecho = true
	return nil
}

func (s *staged) Discard() error {
	if s.hecho {
		return nil
	}
	s.hecho = true
	if err := os.Remove(s.ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// lectorConContexto corta la copia si la peticion se cancela. Sin esto, un
// cliente que se va a mitad de una subida lenta deja la copia esperando hasta
// que el servidor cierre la conexion por su cuenta.
type lectorConContexto struct {
	ctx context.Context
	r   io.Reader
}

func (l lectorConContexto) Read(p []byte) (int, error) {
	if err := l.ctx.Err(); err != nil {
		return 0, err
	}
	return l.r.Read(p)
}
