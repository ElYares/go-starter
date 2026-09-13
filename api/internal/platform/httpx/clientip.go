package httpx

import (
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
func ClientIP(r *http.Request) string {
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
