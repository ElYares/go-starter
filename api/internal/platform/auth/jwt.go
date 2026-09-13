// Package auth es la sesion: el JWT de la cookie `at`, las cuatro cookies que
// fija la Decision 007, el doble envio del CSRF y el limite de intentos.
//
// No conoce ningun modulo, y no puede: es plataforma. Lo que necesita saber de
// identidad —quien es el usuario `sub` y que permisos tiene— entra por la
// funcion ResolverActor, que inyecta `app`. Ver docs/01-arquitectura.md.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// VidaDelAccessToken es corta a proposito: el `at` no se puede revocar —no hay
// nada que consultar, esa es la gracia de un token firmado— asi que lo unico
// que acota el dano de uno robado es que caduque. Quince minutos es el numero
// de la Decision 007, y quien lo suba tiene que decir contra que lo cambia.
const VidaDelAccessToken = 15 * time.Minute

// Claims es lo que viaja dentro del `at`. Son tres campos y no mas: cada uno
// que se agregue es una copia de la base que camina quince minutos desactuali-
// zada. Los permisos NO estan aqui a proposito —se resuelven por peticion— para
// que quitarle un permiso a alguien tenga efecto ya y no en el proximo refresh.
type Claims struct {
	Sub   string   `json:"sub"`
	Roles []string `json:"roles"`
	Exp   int64    `json:"exp"`
}

// Los errores son sentinelas para que quien verifica pueda distinguir "caducado"
// —que es normal y lo arregla un refresh— de "esto no lo firmamos nosotros",
// que no lo es. Se distinguen en el log, nunca en la respuesta: al cliente le
// llega el mismo 401 en los dos casos.
var (
	ErrTokenMalFormado = errors.New("auth: el token no tiene tres partes")
	ErrFirmaInvalida   = errors.New("auth: la firma no coincide")
	ErrAlgoritmo       = errors.New("auth: el algoritmo del token no es HS256")
	ErrExpirado        = errors.New("auth: el token expiro")
	ErrSinSujeto       = errors.New("auth: el token no dice de quien es")
)

// Firmante emite y verifica los `at`. Guarda la llave, asi que hay uno solo y
// lo arma `app` desde config.
type Firmante struct {
	llave []byte
	ahora func() time.Time
}

// NewFirmante recibe la llave ya leida del entorno: config es el unico paquete
// que traduce variables. Una llave vacia es un error de arranque, no un token
// firmado con nada.
func NewFirmante(llave string) (*Firmante, error) {
	if strings.TrimSpace(llave) == "" {
		return nil, errors.New("auth: la llave de firma esta vacia")
	}
	return &Firmante{llave: []byte(llave), ahora: time.Now}, nil
}

// cabecera es constante: siempre HS256, siempre JWT. Se serializa una vez.
//
// Que sea fija es media defensa contra la confusion de algoritmos; la otra
// mitad —la que importa— es que Verificar la compara con lo que llega en vez de
// leer el `alg` del token para decidir con que verificar. Un verificador que
// obedece al `alg` del propio token acepta `none` y acepta que le cambien HS256
// por RS256 usando la clave publica como secreto.
var cabeceraCodificada = codificar([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Firmar emite un `at` para un usuario y sus roles.
func (f *Firmante) Firmar(sub string, roles []string) (string, error) {
	if sub == "" {
		return "", ErrSinSujeto
	}
	if roles == nil {
		// Un `null` en el JSON y una lista vacia se leen distinto al otro lado.
		roles = []string{}
	}

	cuerpo, err := json.Marshal(Claims{
		Sub:   sub,
		Roles: roles,
		Exp:   f.ahora().Add(VidaDelAccessToken).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("auth: no se pudo serializar el token: %w", err)
	}

	firmado := cabeceraCodificada + "." + codificar(cuerpo)
	return firmado + "." + codificar(f.firma(firmado)), nil
}

// Verificar devuelve los claims de un token que firmamos nosotros y que sigue
// vigente. Cualquier otra cosa es un error.
//
// El orden importa: primero el algoritmo, despues la firma, y solo entonces se
// mira el contenido. Leer los claims de un token cuya firma todavia no se
// comprobo es confiar en datos del atacante, aunque sea "solo para el log".
func (f *Firmante) Verificar(token string) (Claims, error) {
	partes := strings.Split(token, ".")
	if len(partes) != 3 {
		return Claims{}, ErrTokenMalFormado
	}

	// Comparacion literal de la cabecera. No se decodifica ni se interpreta:
	// esto rechaza de una vez `{"alg":"none"}`, `{"alg":"RS256"}` y cualquier
	// cabecera con campos de mas o con los mismos en otro orden.
	if partes[0] != cabeceraCodificada {
		return Claims{}, ErrAlgoritmo
	}

	firmaRecibida, err := decodificar(partes[2])
	if err != nil {
		return Claims{}, ErrFirmaInvalida
	}

	// hmac.Equal y no ==: comparar firmas byte a byte con cortocircuito filtra
	// cuantos bytes iniciales acerto quien esta probando, y con eso se forja
	// una firma valida byte por byte.
	if !hmac.Equal(firmaRecibida, f.firma(partes[0]+"."+partes[1])) {
		return Claims{}, ErrFirmaInvalida
	}

	cuerpo, err := decodificar(partes[1])
	if err != nil {
		return Claims{}, ErrTokenMalFormado
	}

	var c Claims
	if err := json.Unmarshal(cuerpo, &c); err != nil {
		return Claims{}, ErrTokenMalFormado
	}

	switch {
	case c.Sub == "":
		return Claims{}, ErrSinSujeto
	// Sin Exp, `c.Exp` es cero y esta comparacion lo rechaza: un token sin
	// caducidad es un token eterno, que es justo lo que la vida corta evita.
	case !f.ahora().Before(time.Unix(c.Exp, 0)):
		return Claims{}, ErrExpirado
	}

	return c, nil
}

func (f *Firmante) firma(mensaje string) []byte {
	m := hmac.New(sha256.New, f.llave)
	m.Write([]byte(mensaje))
	return m.Sum(nil)
}

// base64url sin relleno, que es lo que dice el RFC 7515. Con relleno, el `=`
// tendria que ir escapado en la cookie y ningun otro cliente leeria el token.
func codificar(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func decodificar(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
