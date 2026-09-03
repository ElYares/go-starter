package audit

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// La prueba vive dentro del paquete para poder congelar el reloj. Sin eso, lo
// unico que se podria afirmar del sello de tiempo es que no es cero, y eso lo
// cumple tambien un reloj equivocado.
func conReloj(t *testing.T, instante time.Time) {
	t.Helper()
	anterior := ahora
	ahora = func() time.Time { return instante }
	t.Cleanup(func() { ahora = anterior })
}

const juan = "3f1c9b2e-7d5a-4c81-9e0f-2a6b8c4d1e33"

func ctxCon(id string) context.Context {
	return rbac.WithActor(context.Background(), rbac.Actor{ID: id})
}

func uuidDe(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("uuid de prueba invalido: %v", err)
	}
	return u
}

func TestElAltaSellaLasCuatroColumnas(t *testing.T) {
	instante := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	conReloj(t, instante)

	f, err := ForInsert(ctxCon(juan))
	if err != nil {
		t.Fatalf("ForInsert: %v", err)
	}

	esperadas := []string{"created_at", "created_by", "updated_at", "updated_by"}
	if !slices.Equal(f.Columns, esperadas) {
		t.Fatalf("columnas = %v, se esperaban %v", f.Columns, esperadas)
	}
	if len(f.Values) != 4 {
		t.Fatalf("valores = %#v", f.Values)
	}

	if f.Values[0] != instante || f.Values[2] != instante {
		t.Errorf("los sellos de tiempo = %v / %v, se esperaba %v", f.Values[0], f.Values[2], instante)
	}
	quien := uuidDe(t, juan)
	if f.Values[1] != quien || f.Values[3] != quien {
		t.Errorf("el actor no quedo en las dos columnas: %#v", f.Values)
	}
}

// Una fila recien creada cuyo updated_at fuera distinto del created_at mentiria
// sobre haber sido modificada.
func TestEnUnAltaCreacionYModificacionSonElMismoInstante(t *testing.T) {
	conReloj(t, time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC))

	f, _ := ForInsert(ctxCon(juan))
	if f.Values[0] != f.Values[2] {
		t.Errorf("created_at = %v, updated_at = %v; en un alta tienen que coincidir", f.Values[0], f.Values[2])
	}
}

// El corazon del asunto: una modificacion no puede tocar quien creo la fila.
func TestLaModificacionNoTocaLasColumnasDeCreacion(t *testing.T) {
	f, err := ForUpdate(ctxCon(juan))
	if err != nil {
		t.Fatalf("ForUpdate: %v", err)
	}

	esperadas := []string{"updated_at", "updated_by"}
	if !slices.Equal(f.Columns, esperadas) {
		t.Fatalf("columnas = %v, se esperaban %v", f.Columns, esperadas)
	}
	for _, c := range f.Columns {
		if c == "created_at" || c == "created_by" {
			t.Errorf("una modificacion escribe %q: eso borra quien creo la fila", c)
		}
	}
}

func TestSinActorFirmaElSistemaYNuncaUnNull(t *testing.T) {
	f, err := ForInsert(context.Background())
	if err != nil {
		t.Fatalf("una operacion sin actor no puede fallar: %v", err)
	}

	sistema := uuidDe(t, SystemActor)
	for i, c := range f.Columns {
		if c != "created_by" && c != "updated_by" {
			continue
		}
		u, ok := f.Values[i].(pgtype.UUID)
		if !ok {
			t.Fatalf("%s = %#v; se esperaba un uuid", c, f.Values[i])
		}
		if !u.Valid {
			t.Errorf("%s quedo en NULL; el sistema tiene que firmar explicitamente", c)
		}
		if u != sistema {
			t.Errorf("%s = %v, se esperaba el actor de sistema", c, u)
		}
	}
}

// Atribuir mal una escritura es peor que fallar: la primera no se nota.
func TestUnActorConIdQueNoEsUuidFallaEnVezDeAtribuirseAlSistema(t *testing.T) {
	_, err := ForInsert(ctxCon("juan"))
	if err == nil {
		t.Fatal("un id que no es uuid tiene que ser un error, no un fallback silencioso")
	}
}

func TestUnActorSinIdFalla(t *testing.T) {
	if _, err := ForUpdate(ctxCon("")); err == nil {
		t.Fatal("un actor sin id es un bug del middleware de sesion, no una operacion de sistema")
	}
}

func TestLosFragmentosDeSqlSeNumeranDesdeDondeSeLesDice(t *testing.T) {
	f, _ := ForInsert(ctxCon(juan))

	if got := f.ColumnList(); got != "created_at, created_by, updated_at, updated_by" {
		t.Errorf("columnList = %q", got)
	}
	if got := f.Placeholders(3); got != "$3, $4, $5, $6" {
		t.Errorf("placeholders = %q", got)
	}

	u, _ := ForUpdate(ctxCon(juan))
	if got := u.Assignments(2); got != "updated_at = $2, updated_by = $3" {
		t.Errorf("assignments = %q", got)
	}
}
