package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"
)

// Los nombres y los Path los fija la Decision 007. No son cuatro cookies por
// gusto: cada Path acota donde se manda cada cosa, y dos de ellas existen para
// que el navegador NO mande las otras dos cuando no toca.
const (
	// CookieAT es el JWT. Path=/api: no viaja en la carga de la landing.
	CookieAT = "at"
	// CookieRT es el refresh opaco. Path=/api/v1/auth y solo ahi: si viajara en
	// cada peticion, cada peticion seria una oportunidad de robarlo.
	CookieRT = "rt"
	// CookieCSRF la lee JavaScript a proposito —es la mitad del doble envio— y
	// va en Path=/ porque en /api la SPA no la ve. El sintoma de equivocarse
	// aqui es un 403 en el primer POST y ninguna pista de por que.
	CookieCSRF = "XSRF-TOKEN"
	// CookieHasSession es una pista, no una credencial: dice "hubo sesion" para
	// que una carga anonima de la landing no dispare un refresh que responde
	// 401. Legible por JS y sin ningun valor para quien la falsifique: lo unico
	// que consigue es pedir un refresh que le van a negar.
	CookieHasSession = "has_session"
)

const (
	pathAPI  = "/api"
	pathAuth = "/api/v1/auth"
	pathRaiz = "/"

	// VidaDelRefreshToken y la de la pista van juntas a proposito: una pista que
	// dura mas que el refresh manda a pedir renovaciones que ya no pueden salir.
	VidaDelRefreshToken = 14 * 24 * time.Hour
)

// Emisor escribe las cookies de sesion. `secure` sale de config y no se deduce
// del esquema de la peticion: detras del edge todo llega por http, asi que
// mirar r.TLS apagaria el flag justo en produccion, que es donde importa.
type Emisor struct{ secure bool }

func NewEmisor(secure bool) *Emisor { return &Emisor{secure: secure} }

// Emitir pone las cuatro cookies de una sesion recien abierta o recien rotada.
func (e *Emisor) Emitir(w http.ResponseWriter, at, rt, csrf string) {
	http.SetCookie(w, e.cookie(CookieAT, at, pathAPI, VidaDelAccessToken, true))
	http.SetCookie(w, e.cookie(CookieRT, rt, pathAuth, VidaDelRefreshToken, true))
	// De sesion —MaxAge 0— porque su vida util es la de la pestana: rota en cada
	// renovacion y el cliente la relee de la cookie en cada peticion.
	http.SetCookie(w, e.cookie(CookieCSRF, csrf, pathRaiz, 0, false))
	http.SetCookie(w, e.cookie(CookieHasSession, "1", pathRaiz, VidaDelRefreshToken, false))
}

// Limpiar borra las cuatro. Cada una tiene que borrarse con SU MISMO Path: un
// borrado en Path=/ no toca la cookie que se puso en Path=/api, y el navegador
// se queda con un `at` invisible que sigue mandando.
func (e *Emisor) Limpiar(w http.ResponseWriter) {
	http.SetCookie(w, e.vencida(CookieAT, pathAPI, true))
	http.SetCookie(w, e.vencida(CookieRT, pathAuth, true))
	http.SetCookie(w, e.vencida(CookieCSRF, pathRaiz, false))
	http.SetCookie(w, e.vencida(CookieHasSession, pathRaiz, false))
}

// EmitirCSRF pone solo el token CSRF. Lo usa el middleware en la primera lectura
// de quien todavia no lo tiene: sin eso, el primer POST de una sesion nueva
// —incluido el login— no tendria cabecera que mandar.
func (e *Emisor) EmitirCSRF(w http.ResponseWriter, csrf string) {
	http.SetCookie(w, e.cookie(CookieCSRF, csrf, pathRaiz, 0, false))
}

func (e *Emisor) cookie(nombre, valor, path string, vida time.Duration, httpOnly bool) *http.Cookie {
	c := &http.Cookie{
		Name:     nombre,
		Value:    valor,
		Path:     path,
		HttpOnly: httpOnly,
		Secure:   e.secure,
		// Lax y no Strict: con Strict, llegar al dashboard desde un enlace de
		// correo no manda las cookies y el usuario ve la pantalla de login
		// teniendo sesion. Lax ya corta el envio en las peticiones de otro
		// origen que importan, que son las mutaciones.
		SameSite: http.SameSiteLaxMode,
	}
	if vida > 0 {
		c.MaxAge = int(vida.Seconds())
	}
	return c
}

func (e *Emisor) vencida(nombre, path string, httpOnly bool) *http.Cookie {
	c := e.cookie(nombre, "", path, 0, httpOnly)
	// MaxAge negativo es "borrala ya". Un MaxAge 0 la deja de sesion, que es
	// justo lo contrario de lo que quiere un borrado.
	c.MaxAge = -1
	return c
}

// NuevoRefreshToken devuelve el valor opaco que va en la cookie `rt`.
//
// Opaco y no un JWT: el refresh SI se puede revocar, y para eso hay que poder
// buscarlo en la base. 256 bits de aleatoriedad no se adivinan, y no dicen nada
// de quien lo tiene.
func NuevoRefreshToken() (string, error) { return aleatorio(32) }

// NuevoTokenCSRF no necesita ser impredecible para siempre, pero si tiene que
// serlo para quien intenta forjar la cabecera. Sale de la misma fuente.
func NuevoTokenCSRF() (string, error) { return aleatorio(32) }

// HashDeRefresh es lo que se guarda en la base, nunca el token.
//
// SHA-256 pelado y no argon2: aqui no hace falta encarecer el intento porque no
// hay nada que adivinar —son 256 bits aleatorios, no una contrasena que alguien
// eligio— y el hash se consulta en cada renovacion. Ver la Decision 007.
func HashDeRefresh(token string) []byte {
	suma := sha256.Sum256([]byte(token))
	return suma[:]
}

func aleatorio(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
