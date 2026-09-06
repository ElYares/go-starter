package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// El hash de contrasenas vive dentro del modulo, no en platform, por la misma
// razon que sus migraciones: borrar la carpeta de identity tiene que llevarse
// todo lo que solo existia para el. Un paquete de plataforma con un unico
// consumidor es una dependencia que parece compartida y no lo es.

// Parametros de argon2id, en las recomendaciones del RFC 9106 para el perfil de
// memoria alta. No son constantes por gusto: quedan escritos DENTRO de cada
// hash, asi que subirlos no invalida las contrasenas existentes —siguen
// verificandose con los suyos— y solo afecta a las que se guarden despues.
const (
	memoriaKiB  = 64 * 1024
	iteraciones = 3
	paralelismo = 2
	largoSal    = 16
	largoHash   = 32
)

// Un hash mal formado es un error, no un "no coincide". Distinguirlos importa:
// lo primero es una fila corrupta que hay que ver en el log, lo segundo es un
// usuario que se equivoco de contrasena.
var (
	errHashInvalido = errors.New("identity: el hash guardado no tiene el formato argon2id esperado")
	errHashVersion  = errors.New("identity: el hash guardado usa una version de argon2 que este binario no conoce")
)

// techoMemoriaKiB acota lo que se acepta al LEER un hash. El coste de verificar
// lo dicta el propio hash, asi que una fila con m=16777216 haria que un intento
// de login reservara 16 GiB. Con el tope, esa fila falla en vez de tumbar el
// proceso.
const techoMemoriaKiB = 1 << 20 // 1 GiB

// Hash devuelve la contrasena en formato PHC:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<sal>$<hash>
//
// El formato no es decorativo: lleva los parametros dentro, que es lo que
// permite cambiarlos sin migrar la tabla ni dejar a nadie fuera.
func Hash(password string) (string, error) {
	sal := make([]byte, largoSal)
	if _, err := rand.Read(sal); err != nil {
		return "", fmt.Errorf("identity: no se pudo generar la sal: %w", err)
	}

	suma := argon2.IDKey([]byte(password), sal, iteraciones, memoriaKiB, paralelismo, largoHash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoriaKiB, iteraciones, paralelismo,
		base64.RawStdEncoding.EncodeToString(sal),
		base64.RawStdEncoding.EncodeToString(suma),
	), nil
}

// Verify compara en tiempo constante.
//
// Un `==` sobre los bytes sale antes en cuanto encuentra una diferencia, y esa
// diferencia de microsegundos es medible por la red: basta para adivinar el
// hash byte a byte. subtle.ConstantTimeCompare tarda lo mismo siempre.
func Verify(password, codificado string) (bool, error) {
	sal, esperado, err := descomponer(codificado)
	if err != nil {
		return false, err
	}

	m, t, p, err := parametros(codificado)
	if err != nil {
		return false, err
	}

	suma := argon2.IDKey([]byte(password), sal, t, m, p, uint32(len(esperado)))
	return subtle.ConstantTimeCompare(suma, esperado) == 1, nil
}

func descomponer(codificado string) (sal, suma []byte, err error) {
	partes := strings.Split(codificado, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", sal, hash
	if len(partes) != 6 || partes[0] != "" {
		return nil, nil, errHashInvalido
	}
	if partes[1] != "argon2id" {
		// argon2i y argon2d existen y no sirven aqui: el perfil que resiste a
		// la vez GPU y canal lateral es el id.
		return nil, nil, errHashInvalido
	}

	sal, err = base64.RawStdEncoding.DecodeString(partes[4])
	if err != nil {
		return nil, nil, errHashInvalido
	}
	suma, err = base64.RawStdEncoding.DecodeString(partes[5])
	if err != nil {
		return nil, nil, errHashInvalido
	}
	if len(sal) == 0 || len(suma) == 0 {
		return nil, nil, errHashInvalido
	}
	return sal, suma, nil
}

func parametros(codificado string) (m uint32, t uint32, p uint8, err error) {
	partes := strings.Split(codificado, "$")
	if len(partes) != 6 {
		return 0, 0, 0, errHashInvalido
	}

	var version int
	if _, err := fmt.Sscanf(partes[2], "v=%d", &version); err != nil {
		return 0, 0, 0, errHashInvalido
	}
	if version != argon2.Version {
		return 0, 0, 0, errHashVersion
	}

	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return 0, 0, 0, errHashInvalido
	}
	if m == 0 || t == 0 || p == 0 || m > techoMemoriaKiB {
		return 0, 0, 0, errHashInvalido
	}
	return m, t, p, nil
}
