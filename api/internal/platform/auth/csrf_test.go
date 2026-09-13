package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func siguienteQueMarca(llamado *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*llamado = true
		w.WriteHeader(http.StatusOK)
	})
}

func cookieDeLaRespuesta(rec *httptest.ResponseRecorder, nombre string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == nombre {
			return c
		}
	}
	return nil
}

// El primer GET de un visitante nuevo tiene que dejarle la cookie. Sin esto, su
// primer POST —el login— no tendria nada que copiar a la cabecera y recibiria
// un 403 del que no puede salir por si mismo.
func TestUnaLecturaSinCookieLaSiembra(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))

	if !paso {
		t.Fatal("la lectura no llego al handler")
	}

	c := cookieDeLaRespuesta(rec, CookieCSRF)
	if c == nil || c.Value == "" {
		t.Fatal("no se sembro la cookie XSRF-TOKEN")
	}
	// En Path=/ o la SPA no la ve, y el sintoma es un 403 en el primer POST.
	if c.Path != "/" {
		t.Errorf("Path = %q, tiene que ser /", c.Path)
	}
	// Es la unica del juego que JS tiene que poder leer: si fuera HttpOnly, el
	// doble envio no se podria hacer.
	if c.HttpOnly {
		t.Error("la cookie CSRF es HttpOnly: nadie puede copiarla a la cabecera")
	}
}

// Y no se resiembra si ya hay una: rotarla en cada lectura dejaria al cliente
// mandando siempre la anterior, y toda mutacion daria 403.
func TestUnaLecturaConCookieNoLaCambia(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "la-que-ya-tenia"})

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

	if c := cookieDeLaRespuesta(rec, CookieCSRF); c != nil {
		t.Errorf("se resembro la cookie con %q teniendo una", c.Value)
	}
}

func TestUnaMutacionConCookieYCabeceraQueCoincidenPasa(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "el-token"})
	r.Header.Set(CabeceraCSRF, "el-token")

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

	if !paso {
		t.Fatalf("no paso: estado %d", rec.Code)
	}
}

// El caso que este middleware existe para tapar: el navegador manda la cookie
// solo, pero el sitio de otro origen no puede leerla para copiarla.
func TestUnaMutacionSinCabeceraSeRechaza(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "el-token"})

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

	if paso {
		t.Fatal("paso una mutacion sin la cabecera")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("estado = %d, se esperaba 403", rec.Code)
	}
}

// La trampa fina: sin cookie NI cabecera, una comparacion ingenua compara la
// cadena vacia contra la cadena vacia y deja pasar. Es el agujero exacto que
// este middleware existe para tapar, y no se ve leyendo el codigo.
func TestUnaMutacionSinCookieNiCabeceraNoPasaComparandoVacioConVacio(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if paso {
		t.Fatal("paso una mutacion sin cookie ni cabecera: se comparo vacio con vacio")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("estado = %d, se esperaba 403", rec.Code)
	}
}

// Lo mismo con la cookie presente pero vacia, que es lo que queda cuando se
// borro la sesion sin recargar.
func TestUnaMutacionConCookieVaciaSeRechaza(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/settings", nil)
	r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: ""})
	r.Header.Set(CabeceraCSRF, "")

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

	if paso {
		t.Fatal("paso una mutacion con cookie y cabecera vacias")
	}
}

func TestUnaMutacionConCabeceraQueNoCoincideSeRechaza(t *testing.T) {
	var paso bool
	rec := httptest.NewRecorder()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/settings", nil)
	r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "el-token"})
	r.Header.Set(CabeceraCSRF, "otro-token")

	CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

	if paso {
		t.Fatal("paso una mutacion con una cabecera que no coincide")
	}
}

// Las cuatro mutaciones cierran, no solo POST. Un DELETE sin comprobar seria
// el mismo agujero con otro verbo.
func TestTodosLosMetodosQueCambianAlgoExigenLaCabecera(t *testing.T) {
	for _, metodo := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		var paso bool
		rec := httptest.NewRecorder()

		r := httptest.NewRequest(metodo, "/api/v1/settings/x", nil)
		r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "el-token"})

		CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).ServeHTTP(rec, r)

		if paso {
			t.Errorf("%s paso sin la cabecera", metodo)
		}
	}
}

// Y los seguros no la exigen. Pedirsela a un preflight romperia todo lo que no
// sea una peticion simple.
func TestLosMetodosSegurosNoExigenNada(t *testing.T) {
	for _, metodo := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		var paso bool
		rec := httptest.NewRecorder()

		CSRF(NewEmisor(true))(siguienteQueMarca(&paso)).
			ServeHTTP(rec, httptest.NewRequest(metodo, "/api/v1/settings", nil))

		if !paso {
			t.Errorf("%s no paso: estado %d", metodo, rec.Code)
		}
	}
}
