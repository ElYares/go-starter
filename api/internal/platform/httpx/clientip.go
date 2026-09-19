package httpx

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// ClientIP devuelve la IP de quien hace la peticion, vista desde detras del
// edge.
//
// **Se toma la ULTIMA entrada de X-Forwarded-For, no la primera.** La cabecera
// la escribe cualquiera: un cliente puede mandar `X-Forwarded-For: 1.2.3.4` y
// Caddy no la borra, la extiende con la IP real de la conexion que recibio. El
// resultado es `1.2.3.4, <ip real>`, asi que la primera entrada es la que eligio
// el atacante y la ultima es la unica que puso alguien de confianza.
//
// Leer la primera —que es lo que hace casi todo el mundo, porque en la cabecera
// "el cliente original va primero"— convierte el limite de intentos en un
// adorno: basta cambiar la cabecera en cada peticion para tener una IP nueva.
//
// Esto vale porque hay EXACTAMENTE UN salto de confianza, el edge. Un fork que
// meta otro proxy delante tiene que contar saltos desde el final, o volvera a
// creerle a quien no debe. Ver docs/08-infra-local.md.
//
// La excepcion es el SSR de Nuxt, que no pasa por el edge: si ConfiarEnSSR
// acredito la peticion, la IP es la del visitante que el SSR dice atender.
func ClientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(claveIPDelSSR{}).(string); ok {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		partes := strings.Split(xff, ",")
		ultima := strings.TrimSpace(partes[len(partes)-1])
		if ip := net.ParseIP(ultima); ip != nil {
			return ip.String()
		}
	}

	// Sin la cabecera —una peticion directa al contenedor, o las pruebas— queda
	// la conexion. Se le quita el puerto: si no, cada peticion del mismo cliente
	// tendria una clave distinta y no habria limite que contara.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Las dos cabeceras con las que el SSR de Nuxt dice a quien atiende.
const (
	CabeceraSecretoSSR = "X-SSR-Secret"
	CabeceraIPDelSSR   = "X-SSR-Client-IP"
)

type claveIPDelSSR struct{}

// ConfiarEnSSR acredita las peticiones del SSR de Nuxt, que le pega al api por
// la red interna y no por el edge (HU-010).
//
// Sin esto, todas las visitas a la landing llegan con la IP del contenedor web
// y comparten un solo balde del limite: el primer pico de trafico la tumbaria
// entera con 429. Con esto, cada visita cuenta contra la IP de su visitante.
//
// **Se confia por un secreto y no por la red.** El edge tambien esta en la red
// interna, asi que "confiar en IPs privadas" le daria el mismo credito a
// cualquier cliente que llegue por el. Un cliente puede mandar las dos
// cabeceras a traves del edge, pero no conoce el secreto.
//
// Las dos cabeceras se BORRAN siempre, coincida o no el secreto: nada despues
// de este middleware —el log incluido— tiene por que ver el secreto, y un
// handler que las leyera a mano se saltaria esta comprobacion.
//
// Un secreto vacio no acredita nada. config lo exige, pero una prueba que arme
// la cadena sin el no tiene que abrir la puerta.
func ConfiarEnSSR(secreto string) func(http.Handler) http.Handler {
	esperado := []byte(secreto)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dado := r.Header.Get(CabeceraSecretoSSR)
			ip := net.ParseIP(strings.TrimSpace(r.Header.Get(CabeceraIPDelSSR)))
			r.Header.Del(CabeceraSecretoSSR)
			r.Header.Del(CabeceraIPDelSSR)

			// ConstantTimeCompare y no ==: una comparacion que sale en el
			// primer byte distinto deja adivinar el secreto midiendo tiempos.
			if len(esperado) > 0 && ip != nil &&
				subtle.ConstantTimeCompare([]byte(dado), esperado) == 1 {
				r = r.WithContext(context.WithValue(r.Context(), claveIPDelSSR{}, ip.String()))
			}
			next.ServeHTTP(w, r)
		})
	}
}
