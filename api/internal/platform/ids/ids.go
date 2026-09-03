// Package ids genera las llaves primarias del proyecto.
//
// Existe para que la convencion de docs/03-modelo-de-datos.md —uuid v7
// generado en Go, no `serial`— sea una llamada y no un recordatorio. Un id
// secuencial filtra volumen de negocio (el pedido 412 dice cuantos hubo antes)
// y estorba el dia que hay que fusionar dos bases.
//
// v7 y no v4: los v7 llevan el tiempo en el prefijo, asi que ordenan por
// creacion y no fragmentan el indice al insertarse. Un v4 aleatorio escribe en
// una pagina distinta cada vez.
package ids

import (
	"fmt"

	"github.com/google/uuid"
)

// New devuelve un uuid v7.
//
// Devuelve error en vez de entrar en panico porque la unica causa posible es
// que la fuente de aleatoriedad del sistema falle, y eso es una condicion de
// ejecucion —un contenedor sin /dev/urandom— no un bug del programa.
func New() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("ids: no se pudo generar un uuid v7: %w", err)
	}
	return id, nil
}

// NewString es lo mismo para quien guarda el id como texto.
func NewString() (string, error) {
	id, err := New()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
