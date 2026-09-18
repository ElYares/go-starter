// Package storage guarda archivos por llave, sin saber que contienen.
//
// La base guarda metadatos y la llave; los bytes viven aqui. Es una interfaz
// para que el dia que un fork se vaya a S3 cambie la implementacion y nada mas:
// ningun modulo nombra rutas del disco.
//
// Escribir tiene dos tiempos, Stage y despues Commit, porque quien sube casi
// nunca sabe la llave antes de haber leido el archivo entero: en `media`, la
// llave sale del SHA-256 de los bytes. Lo que esta en Stage no es visible bajo
// ninguna llave, asi que un corte a mitad de subida no deja medio archivo con
// nombre valido.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound es la llave que no existe.
var ErrNotFound = errors.New("storage: la llave no existe")

// ErrInvalidKey es una llave que se sale del almacenamiento o no tiene forma de
// llave. Nunca deberia llegar del cliente, pero si llega no abre nada.
var ErrInvalidKey = errors.New("storage: llave invalida")

type Store interface {
	// Stage copia src a un lugar provisional. Si src falla, no queda nada: el
	// error de src vuelve tal cual, para que quien lo limito sepa reconocerlo.
	Stage(ctx context.Context, src io.Reader) (Staged, error)

	// Open abre lo guardado bajo key, o devuelve ErrNotFound.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// Staged es un archivo escrito que todavia no tiene llave. Hay que terminar
// siempre con Commit o con Discard; Discard despues de Commit no hace nada, asi
// que se puede diferir sin pensar.
type Staged interface {
	// Open relee lo escrito, para inspeccionarlo antes de decidir.
	Open() (io.ReadCloser, error)

	// Commit lo deja bajo key de forma atomica. Si ya habia algo bajo key, se
	// reemplaza: las llaves de contenido (un hash) tienen los mismos bytes.
	Commit(ctx context.Context, key string) error

	// Discard lo borra.
	Discard() error
}
