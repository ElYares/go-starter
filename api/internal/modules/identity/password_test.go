package identity

import (
	"errors"
	"strings"
	"testing"
)

const contrasenaDePrueba = "una contrasena larga de verdad"

func TestElHashSaleEnFormatoPHCConSusParametros(t *testing.T) {
	h, err := Hash(contrasenaDePrueba)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// El prefijo entero, no solo "argon2": el formato lleva los parametros
	// dentro, y eso es lo que permite subirlos manana sin dejar a nadie fuera.
	const prefijo = "$argon2id$v=19$m=65536,t=3,p=2$"
	if !strings.HasPrefix(h, prefijo) {
		t.Errorf("hash = %q\nse esperaba que empezara por %q", h, prefijo)
	}
	if partes := strings.Split(h, "$"); len(partes) != 6 {
		t.Errorf("el hash tiene %d segmentos, se esperaban 6: %q", len(partes), h)
	}
}

// La contrasena no puede aparecer dentro del hash, ni entera ni por trozos.
// Parece obvio hasta que alguien "optimiza" guardando el largo o un prefijo.
//
// Solo cuentan los trozos de 6 letras o mas: el hash es base64 aleatorio, y
// una palabra corta como "de" sale ahi por azar en ~1.6% de las corridas. Una
// de 6 letras, en ~1 de cada mil millones.
func TestElHashNoContieneLaContrasena(t *testing.T) {
	const largoMinimoDelTrozo = 6

	h, err := Hash(contrasenaDePrueba)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if strings.Contains(h, contrasenaDePrueba) {
		t.Fatalf("el hash contiene la contrasena en claro: %q", h)
	}
	for _, palabra := range strings.Fields(contrasenaDePrueba) {
		if len(palabra) < largoMinimoDelTrozo {
			continue
		}
		if strings.Contains(h, palabra) {
			t.Errorf("el hash contiene el trozo %q de la contrasena: %q", palabra, h)
		}
	}
}

// Sin sal, dos personas con la misma contrasena tienen el mismo hash, y una
// tabla filtrada dice quienes comparten contrasena. Con sal, no.
func TestDosHashesDeLaMismaContrasenaSonDistintos(t *testing.T) {
	a, err := Hash(contrasenaDePrueba)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := Hash(contrasenaDePrueba)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if a == b {
		t.Fatal("dos hashes de la misma contrasena salieron identicos: falta la sal")
	}

	// Y los dos tienen que verificar. Una sal que no se guarda produce hashes
	// distintos que ademas no verifican, y esta prueba lo separa de aquello.
	for i, h := range []string{a, b} {
		ok, err := Verify(contrasenaDePrueba, h)
		if err != nil {
			t.Fatalf("Verify del hash %d: %v", i, err)
		}
		if !ok {
			t.Errorf("el hash %d no verifica contra su propia contrasena", i)
		}
	}
}

func TestVerifyRechazaLaContrasenaEquivocada(t *testing.T) {
	h, err := Hash(contrasenaDePrueba)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// La ultima es la buena con un caracter de mas: si la comparacion se
	// hiciera por prefijo, esta pasaria.
	for _, mala := range []string{
		"",
		"otra contrasena larga de verdad",
		strings.ToUpper(contrasenaDePrueba),
		contrasenaDePrueba + "x",
	} {
		ok, err := Verify(mala, h)
		if err != nil {
			t.Fatalf("Verify(%q): %v", mala, err)
		}
		if ok {
			t.Errorf("Verify(%q) dijo que si", mala)
		}
	}
}

// Un hash roto no es "contrasena mala": es una fila corrupta que hay que ver en
// el log. Devolver false los confundiria, y el sintoma seria un usuario que no
// puede entrar y un log que no dice nada.
func TestVerifyDistingueUnHashRotoDeUnaContrasenaMala(t *testing.T) {
	casos := map[string]string{
		"vacio":                 "",
		"sin formato":           "solo-texto",
		"sin la sal":            "$argon2id$v=19$m=65536,t=3,p=2$$c2FsdWRvcw",
		"con segmentos de mas":  "$argon2id$v=19$m=65536,t=3,p=2$c2Fs$aGFzaA$sobra",
		"base64 invalido":       "$argon2id$v=19$m=65536,t=3,p=2$!!!!$aGFzaA",
		"otra variante argon2":  "$argon2i$v=19$m=65536,t=3,p=2$c2Fs$aGFzaA",
		"parametros en cero":    "$argon2id$v=19$m=0,t=3,p=2$c2Fs$aGFzaA",
		"parametros ilegibles":  "$argon2id$v=19$memoria=alta$c2Fs$aGFzaA",
		"version desconocida":   "$argon2id$v=13$m=65536,t=3,p=2$c2Fs$aGFzaA",
		"memoria por las nubes": "$argon2id$v=19$m=16777216,t=3,p=2$c2Fs$aGFzaA",
	}

	for nombre, roto := range casos {
		t.Run(nombre, func(t *testing.T) {
			ok, err := Verify(contrasenaDePrueba, roto)
			if err == nil {
				t.Fatalf("Verify no protesto por un hash %s", nombre)
			}
			if ok {
				t.Error("Verify dijo que si con un hash roto")
			}
			if !errors.Is(err, errHashInvalido) && !errors.Is(err, errHashVersion) {
				t.Errorf("error = %v; se esperaba uno de los sentinelas del paquete", err)
			}
		})
	}
}

// El tope de memoria es lo que impide que una fila con m=16 GiB convierta un
// intento de login en una reserva que tumba el proceso. La prueba de arriba lo
// cubre por el resultado; esta fija el limite, que es lo que se rompe si
// alguien lo sube "para probar algo".
func TestElTechoDeMemoriaEsElQueSeDocumenta(t *testing.T) {
	if techoMemoriaKiB != 1<<20 {
		t.Errorf("techoMemoriaKiB = %d KiB; se documenta 1 GiB", techoMemoriaKiB)
	}
	if memoriaKiB > techoMemoriaKiB {
		t.Error("los hashes que este binario genera no pasarian su propia verificacion")
	}
}
