package ids

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewDevuelveLaVersion7(t *testing.T) {
	id, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := id.Version(); got != 7 {
		t.Errorf("version = %d, se esperaba 7: un v4 no ordena por creacion y fragmenta el indice", got)
	}
	if got := id.Variant(); got != uuid.RFC4122 {
		t.Errorf("variante = %v, se esperaba RFC4122", got)
	}
}

// El prefijo temporal es la razon de elegir v7. Si dos ids generados en orden
// no se ordenan igual como texto, la propiedad se perdio y con ella el motivo.
func TestLosIdsOrdenanPorCreacion(t *testing.T) {
	const cuantos = 50

	anterior, err := NewString()
	if err != nil {
		t.Fatalf("NewString: %v", err)
	}

	for i := 1; i < cuantos; i++ {
		actual, err := NewString()
		if err != nil {
			t.Fatalf("NewString: %v", err)
		}
		if actual <= anterior {
			t.Fatalf("el id %d (%s) no es mayor que el anterior (%s)", i, actual, anterior)
		}
		anterior = actual
	}
}

func TestDosIdsNoSonIguales(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a == b {
		t.Error("dos ids consecutivos salieron iguales")
	}
}
