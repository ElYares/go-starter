package settings

import (
	"context"

	"github.com/google/uuid"
)

// Medios es lo que settings necesita de las imagenes: saber si un id existe.
// Se declara aqui, del lado del consumidor, y app lo conecta con el modulo
// media. Un modulo nunca importa otro. Ver docs/02-modulos.md.
//
// Un fork sin imagenes pasa nil: el `format: media-id` se sigue exigiendo como
// uuid, pero no se comprueba que exista.
type Medios interface {
	Existe(ctx context.Context, id uuid.UUID) (bool, error)
}
