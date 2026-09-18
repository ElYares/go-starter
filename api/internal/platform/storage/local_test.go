package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func local(t *testing.T) *Local {
	t.Helper()
	l, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return l
}

// leer toma los dos valores de Open tal cual. Un error sale como texto para que
// la comparacion que lo usa falle mostrandolo.
func leer(rc io.ReadCloser, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

// provisionales cuenta lo que hay en el directorio de trabajo. Es lo que
// delata una subida cortada que dejo basura.
func provisionales(t *testing.T, l *Local) int {
	t.Helper()
	entradas, err := os.ReadDir(filepath.Join(l.raiz, tmpDir))
	if err != nil {
		t.Fatalf("leyendo %s: %v", tmpDir, err)
	}
	return len(entradas)
}

func TestLoGuardadoSeLeeBajoSuLlave(t *testing.T) {
	l := local(t)
	ctx := context.Background()

	s, err := l.Stage(ctx, strings.NewReader("hola"))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := s.Commit(ctx, "ab/abcdef"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if got := leer(l.Open(ctx, "ab/abcdef")); got != "hola" {
		t.Errorf("leido %q", got)
	}
	if n := provisionales(t, l); n != 0 {
		t.Errorf("quedaron %d provisionales tras el commit", n)
	}
}

// Lo provisional no existe bajo ninguna llave: es lo que impide que un corte a
// mitad de subida deje medio archivo con nombre valido.
func TestLoProvisionalNoSeVeHastaElCommit(t *testing.T) {
	l := local(t)
	ctx := context.Background()

	s, err := l.Stage(ctx, strings.NewReader("hola"))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if got := leer(s.Open()); got != "hola" {
		t.Errorf("releido %q", got)
	}
	if _, err := l.Open(ctx, "ab/abcdef"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open antes del commit: %v, se esperaba ErrNotFound", err)
	}

	if err := s.Discard(); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if n := provisionales(t, l); n != 0 {
		t.Errorf("Discard dejo %d provisionales", n)
	}
	// Discard despues de Discard no falla: se difiere sin pensar.
	if err := s.Discard(); err != nil {
		t.Errorf("segundo Discard: %v", err)
	}
}

type lectorQueFalla struct{ err error }

func (l lectorQueFalla) Read([]byte) (int, error) { return 0, l.err }

// El error del lector vuelve tal cual: es como `media` distingue "demasiado
// grande" de un fallo del disco.
func TestSiElOrigenFallaNoQuedaNadaYElErrorVuelveTalCual(t *testing.T) {
	l := local(t)
	propio := errors.New("demasiado grande")

	_, err := l.Stage(context.Background(), io.MultiReader(strings.NewReader("empieza"), lectorQueFalla{propio}))
	if !errors.Is(err, propio) {
		t.Fatalf("err = %v, se esperaba el del origen", err)
	}
	if n := provisionales(t, l); n != 0 {
		t.Errorf("quedaron %d provisionales", n)
	}
}

func TestUnaPeticionCanceladaCortaLaCopia(t *testing.T) {
	l := local(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := l.Stage(ctx, strings.NewReader("hola"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, se esperaba context.Canceled", err)
	}
	if n := provisionales(t, l); n != 0 {
		t.Errorf("quedaron %d provisionales", n)
	}
}

// Una llave de contenido se puede repetir: dos subidas del mismo archivo
// terminan en el mismo lugar, con los mismos bytes.
func TestCommitSobreUnaLlaveExistenteLaReemplaza(t *testing.T) {
	l := local(t)
	ctx := context.Background()

	for range 2 {
		s, err := l.Stage(ctx, strings.NewReader("mismo"))
		if err != nil {
			t.Fatalf("Stage: %v", err)
		}
		if err := s.Commit(ctx, "ab/abcdef"); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	if got := leer(l.Open(ctx, "ab/abcdef")); got != "mismo" {
		t.Errorf("leido %q", got)
	}
}

func TestUnaLlaveNoPuedeSalirseDeLaRaiz(t *testing.T) {
	l := local(t)
	ctx := context.Background()

	for _, key := range []string{"../fuera", "/etc/passwd", "ab/../../fuera", "", "ab//cd", `ab\cd`, ".tmp/subida-1", "AB"} {
		if _, err := l.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Open(%q) = %v, se esperaba ErrInvalidKey", key, err)
		}
		s, err := l.Stage(ctx, strings.NewReader("x"))
		if err != nil {
			t.Fatalf("Stage: %v", err)
		}
		if err := s.Commit(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Commit(%q) = %v, se esperaba ErrInvalidKey", key, err)
		}
		_ = s.Discard()
	}
}

func TestSinRaizNoSeArranca(t *testing.T) {
	if _, err := NewLocal(""); err == nil {
		t.Error("NewLocal(\"\") no fallo")
	}
	archivo := filepath.Join(t.TempDir(), "soy-un-archivo")
	if err := os.WriteFile(archivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocal(archivo); err == nil {
		t.Error("NewLocal sobre un archivo no fallo")
	}
}
