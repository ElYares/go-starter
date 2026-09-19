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

// --- el SSR de Nuxt (HU-010) ---------------------------------------------------

// ipVista pasa la peticion por ConfiarEnSSR y devuelve la IP que ve lo que
// viene despues, y las cabeceras con las que llego.
func ipVista(t *testing.T, secreto string, prepara func(*http.Request)) (string, http.Header) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/settings", nil)
	req.RemoteAddr = "172.18.0.5:41234" // el contenedor web
	prepara(req)

	var (
		ip        string
		cabeceras http.Header
	)
	ConfiarEnSSR(secreto)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ip, cabeceras = ClientIP(r), r.Header.Clone()
	})).ServeHTTP(httptest.NewRecorder(), req)
	return ip, cabeceras
}

func TestConElSecretoSeUsaLaIpDelVisitanteQueDiceElSSR(t *testing.T) {
	ip, cabeceras := ipVista(t, "el-secreto", func(r *http.Request) {
		r.Header.Set(CabeceraSecretoSSR, "el-secreto")
		r.Header.Set(CabeceraIPDelSSR, "203.0.113.7")
	})
	if ip != "203.0.113.7" {
		t.Errorf("ip = %q; con el secreto tiene que ser la del visitante", ip)
	}
	if cabeceras.Get(CabeceraSecretoSSR) != "" || cabeceras.Get(CabeceraIPDelSSR) != "" {
		t.Errorf("las cabeceras del SSR siguieron adelante: %v", cabeceras)
	}
}

// El criterio 6: un cliente que llega por el edge puede mandar las cabeceras
// del SSR, pero sin el secreto no gana nada. Sigue contando la ultima entrada
// de X-Forwarded-For, la que puso el edge.
func TestSinElSecretoLasCabecerasDelSSRNoCuentan(t *testing.T) {
	for nombre, secreto := range map[string]string{"otro": "adivinado", "vacio": "", "casi": "el-secret"} {
		t.Run(nombre, func(t *testing.T) {
			ip, cabeceras := ipVista(t, "el-secreto", func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.9")
				r.Header.Set(CabeceraSecretoSSR, secreto)
				r.Header.Set(CabeceraIPDelSSR, "203.0.113.7")
			})
			if ip != "198.51.100.9" {
				t.Errorf("ip = %q; sin el secreto manda el edge", ip)
			}
			if cabeceras.Get(CabeceraSecretoSSR) != "" {
				t.Error("un secreto equivocado siguio adelante en las cabeceras")
			}
		})
	}
}

// Sin secreto configurado no se acredita a nadie, ni a quien mande uno vacio.
func TestUnSecretoVacioNoAcreditaANadie(t *testing.T) {
	ip, _ := ipVista(t, "", func(r *http.Request) {
		r.Header.Set(CabeceraSecretoSSR, "")
		r.Header.Set(CabeceraIPDelSSR, "203.0.113.7")
	})
	if ip != "172.18.0.5" {
		t.Errorf("ip = %q; sin secreto configurado cuenta la conexion", ip)
	}
}

func TestUnaIpDelSSRQueNoEsIpNoSeCree(t *testing.T) {
	ip, _ := ipVista(t, "el-secreto", func(r *http.Request) {
		r.Header.Set(CabeceraSecretoSSR, "el-secreto")
		r.Header.Set(CabeceraIPDelSSR, "no-soy-una-ip")
	})
	if ip != "172.18.0.5" {
		t.Errorf("ip = %q", ip)
	}
}
