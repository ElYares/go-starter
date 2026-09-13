package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// LA prueba de este archivo. X-Forwarded-For la escribe cualquiera: el cliente
// manda la suya y el edge la EXTIENDE con la IP real de la conexion. Leer la
// primera entrada —que es lo que hace casi todo el mundo, porque "el cliente
// original va primero"— convierte el limite de intentos en un adorno: basta
// cambiar la cabecera en cada peticion para estrenar IP.
func TestSeTomaLaUltimaEntradaDeForwardedForYNoLaPrimera(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.RemoteAddr = "172.18.0.5:41234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.9")

	if ip := ClientIP(r); ip != "203.0.113.9" {
		t.Fatalf("ip = %q; la primera entrada la eligio quien hace la peticion", ip)
	}
}

// Y con una sola entrada, esa es la que puso el edge.
func TestConUnSoloSaltoSeUsaEsaEntrada(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "172.18.0.5:41234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")

	if ip := ClientIP(r); ip != "203.0.113.9" {
		t.Fatalf("ip = %q", ip)
	}
}

// Sin la cabecera queda la conexion, SIN el puerto: con puerto, cada peticion
// del mismo cliente seria una clave distinta y no habria limite que contara.
func TestSinCabeceraSeUsaLaConexionSinElPuerto(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:41234"

	if ip := ClientIP(r); ip != "203.0.113.9" {
		t.Fatalf("ip = %q, se esperaba sin puerto", ip)
	}
}

// Una cabecera con basura no puede colarse como IP: seria una clave de limite
// que el atacante escribe entera.
func TestUnaCabeceraQueNoEsUnaIpCaeALaConexion(t *testing.T) {
	for _, basura := range []string{"no-soy-una-ip", "1.2.3.4, tampoco", ", ", "999.999.999.999"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "203.0.113.9:41234"
		r.Header.Set("X-Forwarded-For", basura)

		if ip := ClientIP(r); ip != "203.0.113.9" {
			t.Errorf("con %q la ip fue %q", basura, ip)
		}
	}
}

func TestUnaIpV6SeResuelveIgual(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[2001:db8::1]:41234"

	if ip := ClientIP(r); ip != "2001:db8::1" {
		t.Fatalf("ip = %q", ip)
	}

	r.Header.Set("X-Forwarded-For", "1.2.3.4, 2001:db8::2")
	if ip := ClientIP(r); ip != "2001:db8::2" {
		t.Fatalf("ip = %q", ip)
	}
}
