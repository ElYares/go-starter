package auth

import (
	"crypto/hmac"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// CabeceraCSRF es donde el cliente reenvia el valor de la cookie.
const CabeceraCSRF = "X-XSRF-TOKEN"

// CSRF exige el doble envio en toda mutacion.
//
// La idea entera: la cookie la manda el navegador sola, pero solo codigo del
// MISMO origen puede leerla para copiarla a la cabecera. Un formulario en otro
// sitio consigue que el navegador mande la cookie —eso es CSRF— y no consigue
// leerla, asi que no puede poner la cabecera.
//
// De ahi que la cookie NO sea HttpOnly. Es la unica del juego que no lo es, y
// no es un descuido: una cookie CSRF que JavaScript no puede leer no sirve para
// el doble envio. Su valor no abre nada por si solo.
//
// En los metodos seguros el middleware no exige nada y ademas SIEMBRA la cookie
// si falta. Sin eso, el primer POST de un visitante nuevo —el login— no tendria
// nada que copiar y responderia 403 sin que el cliente pueda hacer nada.
func CSRF(e *Emisor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieCSRF)

			if esSeguro(r.Method) {
				if err != nil || cookie.Value == "" {
					token, err := NuevoTokenCSRF()
					if err != nil {
						httpx.WriteProblem(w, r, httpx.Internal())
						return
					}
					e.EmitirCSRF(w, token)
				}
				next.ServeHTTP(w, r)
				return
			}

			enviado := r.Header.Get(CabeceraCSRF)

			// Los tres casos dan el mismo 403 y es correcto: al cliente honesto
			// le sobra con "refresca y vuelve a intentar", y al que esta
			// probando no se le regala el mapa de que le falto.
			//
			// El valor vacio se rechaza aparte porque, sin esa comprobacion,
			// una peticion sin cookie NI cabecera compararia "" contra "" y
			// pasaria: el agujero exacto que este middleware existe para tapar.
			if err != nil || cookie.Value == "" || enviado == "" ||
				!hmac.Equal([]byte(cookie.Value), []byte(enviado)) {
				httpx.WriteProblem(w, r, httpx.Forbidden())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// esSeguro son los metodos que no cambian nada. HEAD y OPTIONS entran con GET:
// exigirle a un preflight una cabecera que el navegador no manda romperia todo
// lo que no sea una peticion simple.
func esSeguro(metodo string) bool {
	switch metodo {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
