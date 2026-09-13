package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func emitidas(t *testing.T, fn func(w http.ResponseWriter)) map[string]*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	fn(rec)

	out := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		out[c.Name] = c
	}
	return out
}

// El reparto de Path y de HttpOnly es la Decision 007, y cada casilla tiene una
// razon. Esta prueba es lo que impide que alguien "simplifique" poniendolas
// todas en Path=/ y deje el refresh viajando en cada peticion.
func TestCadaCookieSaleConSuPathYSuBandera(t *testing.T) {
	cookies := emitidas(t, func(w http.ResponseWriter) {
		NewEmisor(true).Emitir(w, "el-at", "el-rt", "el-csrf")
	})

	esperado := map[string]struct {
		path     string
		httpOnly bool
	}{
		// El at no viaja en la carga de la landing.
		CookieAT: {"/api", true},
		// El rt solo va donde se renueva: si viajara en cada peticion, cada
		// peticion seria una oportunidad de robarlo.
		CookieRT: {"/api/v1/auth", true},
		// La CSRF tiene que leerla JS, y en /api la SPA no la ve.
		CookieCSRF: {"/", false},
		// La pista tambien la lee JS, para no pedir un refresh que va a fallar.
		CookieHasSession: {"/", false},
	}

	for nombre, quiero := range esperado {
		c, ok := cookies[nombre]
		if !ok {
			t.Errorf("no se emitio %s", nombre)
			continue
		}
		if c.Path != quiero.path {
			t.Errorf("%s: Path = %q, se esperaba %q", nombre, c.Path, quiero.path)
		}
		if c.HttpOnly != quiero.httpOnly {
			t.Errorf("%s: HttpOnly = %v, se esperaba %v", nombre, c.HttpOnly, quiero.httpOnly)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("%s: SameSite = %v, se esperaba Lax", nombre, c.SameSite)
		}
	}
}

// Secure sale de config y no del esquema de la peticion: detras del edge todo
// llega por http, asi que deducirlo de r.TLS apagaria el flag justo en
// produccion, que es donde importa.
func TestSecureSaleDeLaConfiguracionYNoDeLaPeticion(t *testing.T) {
	con := emitidas(t, func(w http.ResponseWriter) { NewEmisor(true).Emitir(w, "a", "r", "c") })
	for nombre, c := range con {
		if !c.Secure {
			t.Errorf("%s no salio Secure con el emisor en true", nombre)
		}
	}

	sin := emitidas(t, func(w http.ResponseWriter) { NewEmisor(false).Emitir(w, "a", "r", "c") })
	for nombre, c := range sin {
		if c.Secure {
			t.Errorf("%s salio Secure con el emisor en false", nombre)
		}
	}
}

// La CSRF es de sesion —MaxAge 0— porque rota en cada renovacion. Las otras
// tres llevan vida propia.
func TestLaCookieCsrfEsDeSesionYLasDemasTienenVida(t *testing.T) {
	cookies := emitidas(t, func(w http.ResponseWriter) {
		NewEmisor(true).Emitir(w, "el-at", "el-rt", "el-csrf")
	})

	if cookies[CookieCSRF].MaxAge != 0 {
		t.Errorf("la CSRF tiene MaxAge %d y tendria que ser de sesion", cookies[CookieCSRF].MaxAge)
	}
	for _, nombre := range []string{CookieAT, CookieRT, CookieHasSession} {
		if cookies[nombre].MaxAge <= 0 {
			t.Errorf("%s salio con MaxAge %d", nombre, cookies[nombre].MaxAge)
		}
	}
	if cookies[CookieAT].MaxAge >= cookies[CookieRT].MaxAge {
		t.Error("el at tiene que caducar mucho antes que el rt")
	}
}

// Cada cookie se borra con SU MISMO Path. Un borrado en Path=/ no toca la que
// se puso en Path=/api, y el navegador se queda con un `at` invisible que sigue
// mandando: el sintoma es "cerre sesion y sigo dentro".
func TestLimpiarBorraCadaCookieEnSuPropioPath(t *testing.T) {
	cookies := emitidas(t, func(w http.ResponseWriter) { NewEmisor(true).Limpiar(w) })

	paths := map[string]string{
		CookieAT:         "/api",
		CookieRT:         "/api/v1/auth",
		CookieCSRF:       "/",
		CookieHasSession: "/",
	}

	for nombre, path := range paths {
		c, ok := cookies[nombre]
		if !ok {
			t.Errorf("Limpiar no borro %s", nombre)
			continue
		}
		if c.Path != path {
			t.Errorf("%s: se borro en Path %q y se habia puesto en %q", nombre, c.Path, path)
		}
		// Negativo y no cero: un MaxAge 0 la deja de sesion, que es lo
		// contrario de un borrado.
		if c.MaxAge >= 0 {
			t.Errorf("%s: MaxAge = %d, un borrado va con MaxAge negativo", nombre, c.MaxAge)
		}
	}
}

// El refresh se guarda hasheado, y esta prueba afirma lo unico que importa: que
// del hash no se saca el token. Un `HashDeRefresh` que devolviera el valor tal
// cual pasaria cualquier prueba de "guarda algo".
func TestElHashDelRefreshNoSeParecealToken(t *testing.T) {
	token, err := NuevoRefreshToken()
	if err != nil {
		t.Fatalf("NuevoRefreshToken: %v", err)
	}

	h := HashDeRefresh(token)
	if len(h) != 32 {
		t.Errorf("el hash mide %d bytes, SHA-256 son 32", len(h))
	}
	if string(h) == token {
		t.Fatal("el hash es el token")
	}

	// Determinista, o no se podria buscar la fila en la renovacion.
	if string(HashDeRefresh(token)) != string(h) {
		t.Error("el mismo token da hashes distintos")
	}
	if string(HashDeRefresh(token+"x")) == string(h) {
		t.Error("dos tokens distintos dan el mismo hash")
	}
}

// Dos tokens seguidos no se repiten. Es lo unico que separa "256 bits
// aleatorios" de "un contador con nombre bonito".
func TestLosTokensNoSeRepiten(t *testing.T) {
	vistos := map[string]bool{}
	for n := 0; n < 100; n++ {
		rt, err := NuevoRefreshToken()
		if err != nil {
			t.Fatalf("NuevoRefreshToken: %v", err)
		}
		if vistos[rt] {
			t.Fatalf("se repitio un refresh token en la vuelta %d", n)
		}
		vistos[rt] = true

		csrf, err := NuevoTokenCSRF()
		if err != nil {
			t.Fatalf("NuevoTokenCSRF: %v", err)
		}
		if vistos[csrf] {
			t.Fatalf("se repitio un token CSRF en la vuelta %d", n)
		}
		vistos[csrf] = true
	}
}
