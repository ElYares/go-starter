// Package audit llena created_at/created_by/updated_at/updated_by leyendo el
// actor del contexto de la peticion.
//
// El codigo de negocio no los asigna. La razon no es de estilo: si dependiera
// de que alguien se acuerde, la mitad de las filas quedarian sin rastro de
// quien las toco, y eso se descubre el dia que hace falta saberlo, cuando ya no
// se puede reconstruir.
//
// Ver docs/04-reglas-de-crud.md seccion 7.
package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// SystemActor firma lo que no hizo una persona: un seed, una migracion de
// datos, una tarea de fondo.
//
// Es un uuid explicito y no NULL a proposito. "No se sabe quien fue" y "lo hizo
// el sistema" son cosas distintas, y una columna en NULL no las distingue: al
// mirarla meses despues, las dos se leen igual.
const SystemActor = "00000000-0000-0000-0000-000000000000"

// ahora se sustituye en las pruebas. Es la unica manera de afirmar sobre el
// sello de tiempo sin una base de datos, y en CI no hay una.
var ahora = time.Now

// Fields son las columnas de auditoria y sus valores, ya resueltos. El
// repositorio las pega a su SQL sin escribir los nombres: asi no puede
// equivocarse de columna ni olvidarse de una.
type Fields struct {
	Columns []string
	Values  []any
}

// ForInsert sella un alta: las cuatro columnas, con el mismo instante en las
// dos parejas. Una fila recien creada cuyo updated_at fuera distinto del
// created_at mentiria sobre haber sido modificada.
func ForInsert(ctx context.Context) (Fields, error) {
	quien, err := actor(ctx)
	if err != nil {
		return Fields{}, err
	}

	cuando := ahora().UTC()
	return Fields{
		Columns: []string{"created_at", "created_by", "updated_at", "updated_by"},
		Values:  []any{cuando, quien, cuando, quien},
	}, nil
}

// ForUpdate sella una modificacion, y NO toca las columnas de creacion.
//
// Es la mitad del contrato que se rompe sola cuando alguien escribe el UPDATE a
// mano: un `set created_by = ...` en una modificacion borra quien creo la fila,
// no falla, y nadie lo nota.
func ForUpdate(ctx context.Context) (Fields, error) {
	quien, err := actor(ctx)
	if err != nil {
		return Fields{}, err
	}

	return Fields{
		Columns: []string{"updated_at", "updated_by"},
		Values:  []any{ahora().UTC(), quien},
	}, nil
}

// ColumnList devuelve "created_at, created_by, ..." para un INSERT.
func (f Fields) ColumnList() string { return strings.Join(f.Columns, ", ") }

// Placeholders devuelve "$3, $4, $5, $6" para el VALUES de un INSERT,
// numerados a partir de `desde`.
func (f Fields) Placeholders(desde int) string {
	partes := make([]string, len(f.Columns))
	for i := range f.Columns {
		partes[i] = fmt.Sprintf("$%d", desde+i)
	}
	return strings.Join(partes, ", ")
}

// Assignments devuelve "updated_at = $3, updated_by = $4" para el SET de un
// UPDATE, numerados a partir de `desde`.
func (f Fields) Assignments(desde int) string {
	partes := make([]string, len(f.Columns))
	for i, c := range f.Columns {
		partes[i] = fmt.Sprintf("%s = $%d", c, desde+i)
	}
	return strings.Join(partes, ", ")
}

// actor resuelve quien firma. Un id que no es un uuid es un error, no un
// motivo para atribuirselo al sistema: una escritura mal atribuida es peor que
// una escritura que falla, porque la primera no se nota.
func actor(ctx context.Context) (pgtype.UUID, error) {
	id := SystemActor

	if a, ok := rbac.ActorFrom(ctx); ok {
		if a.ID == "" {
			return pgtype.UUID{}, fmt.Errorf("audit: hay un actor en el contexto sin id")
		}
		id = a.ID
	}

	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("audit: el id del actor %q no es un uuid: %w", id, err)
	}
	return u, nil
}
