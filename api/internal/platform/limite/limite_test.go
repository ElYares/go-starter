package limite

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// reloj es un tiempo que la prueba mueve a mano.
type reloj struct{ t time.Time }

func (r *reloj) ahora() time.Time                { return r.t }
func (r *reloj) avanzar(d time.Duration)         { r.t = r.t.Add(d) }
func conReloj(l *Limitador, r *reloj) *Limitador { l.ahora = r.ahora; return l }

const secretoDePrueba = "secreto-de-prueba"

// cadena es la parte de la cadena global que importa aqui, en su orden:
// acreditar al SSR antes de contar, porque el limite lee ClientIP.
func cadena(general, auth *Limitador) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return observ.Chain(ok, observ.TraceID, httpx.ConfiarEnSSR(secretoDePrueba), Middleware(general, auth))
}

// porEdge es una peticion que entro por Caddy: la conexion es la del edge y la
// ultima entrada de X-Forwarded-For es la IP real del cliente.
func porEdge(ruta, ip string, cabeceras ...string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, ruta, nil)
	r.RemoteAddr = "172.18.0.2:50000"
	r.Header.Set("X-Forwarded-For", ip)
	for i := 0; i+1 < len(cabeceras); i += 2 {
		r.Header.Set(cabeceras[i], cabeceras[i+1])
	}
	return r
}

// porSSR es una peticion del SSR de Nuxt: directa al api, desde el contenedor
// web, con el secreto y la IP del visitante que atiende.
func porSSR(ruta, visitante string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, ruta, nil)
	r.RemoteAddr = "172.18.0.5:41234"
	r.Header.Set(httpx.CabeceraSecretoSSR, secretoDePrueba)
	r.Header.Set(httpx.CabeceraIPDelSSR, visitante)
	return r
}

func pedir(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestAlPasarElTopeResponde429ConRetryAfter(t *testing.T) {
	rj := &reloj{t: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)}
	h := cadena(conReloj(New(3, time.Minute), rj), New(3, time.Minute))

	for i := range 3 {
		if rec := pedir(h, porEdge("/api/v1/pages", "203.0.113.7")); rec.Code != http.StatusNoContent {
			t.Fatalf("peticion %d: estado = %d, todavia cabia", i+1, rec.Code)
		}
	}

	rj.avanzar(20 * time.Second)
	rec := pedir(h, porEdge("/api/v1/pages", "203.0.113.7"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("estado = %d, se esperaba 429", rec.Code)
	}
	// Segundos enteros y mayores que cero: faltan 40 para la ventana siguiente.
	if ra := rec.Header().Get("Retry-After"); ra != "40" {
		t.Errorf("Retry-After = %q, se esperaba 40", ra)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != httpx.CodeTooManyRequests || p.TraceID == "" {
		t.Errorf("no es la forma unica de error: %s", rec.Body.String())
	}
}

func TestPasadaLaVentanaVuelveACaber(t *testing.T) {
	rj := &reloj{t: time.Now()}
	l := conReloj(New(1, time.Minute), rj)

	if ok, _ := l.Tomar("a"); !ok {
		t.Fatal("la primera no cupo")
	}
	if ok, _ := l.Tomar("a"); ok {
		t.Fatal("la segunda cupo con tope 1")
	}
	rj.avanzar(time.Minute)
	if ok, _ := l.Tomar("a"); !ok {
		t.Error("en la ventana siguiente no cupo")
	}
}

// El balde de una IP se reinicia al cumplir su ventana, aunque el barrido no
// haya pasado. El barrido va una vez por ventana y a su propio ritmo: una IP
// que empezo a mitad de camino quedaria bloqueada hasta media ventana de mas.
func TestUnaIpVuelveAlCumplirSuVentanaSinEsperarAlBarrido(t *testing.T) {
	rj := &reloj{t: time.Now()}
	l := conReloj(New(1, time.Minute), rj)

	l.Tomar("otra") // el barrido queda fijado aqui
	rj.avanzar(30 * time.Second)
	l.Tomar("a") // la ventana de "a" empieza a mitad de la del barrido
	rj.avanzar(30 * time.Second)
	l.Tomar("otra") // barre: "a" todavia no cumplio y se queda

	rj.avanzar(30 * time.Second) // "a" cumplio su minuto; el barrido, no
	if ok, _ := l.Tomar("a"); !ok {
		t.Error("\"a\" cumplio su ventana y sigue bloqueada hasta el siguiente barrido")
	}
}

// Criterio 2: agotar `/auth` no quita el resto de la API, ni al reves.
func TestLosDosTopesSonIndependientes(t *testing.T) {
	h := cadena(New(2, time.Minute), New(2, time.Minute))
	for range 2 {
		pedir(h, porEdge("/api/v1/auth/login", "203.0.113.7"))
	}
	if rec := pedir(h, porEdge("/api/v1/auth/refresh", "203.0.113.7")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("auth agotado: estado = %d", rec.Code)
	}
	if rec := pedir(h, porEdge("/api/v1/public/settings", "203.0.113.7")); rec.Code != http.StatusNoContent {
		t.Errorf("con /auth agotado, el resto dio %d", rec.Code)
	}
}

// Criterio 3.
func TestUnaIpBloqueadaNoAfectaAOtra(t *testing.T) {
	h := cadena(New(1, time.Minute), New(1, time.Minute))
	pedir(h, porEdge("/api/v1/pages", "203.0.113.7"))
	if rec := pedir(h, porEdge("/api/v1/pages", "203.0.113.7")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("estado = %d", rec.Code)
	}
	if rec := pedir(h, porEdge("/api/v1/pages", "198.51.100.9")); rec.Code != http.StatusNoContent {
		t.Errorf("otra IP dio %d", rec.Code)
	}
}

// Criterio 4: el SSR atiende a muchos visitantes desde la misma conexion.
// Juntos superan el tope de una IP y ninguno recibe 429.
func TestLosVisitantesDelSSRCuentanPorSeparado(t *testing.T) {
	h := cadena(New(2, time.Minute), New(2, time.Minute))
	for i := range 50 {
		visitante := fmt.Sprintf("203.0.113.%d", i+1)
		if rec := pedir(h, porSSR("/api/v1/public/pages/inicio", visitante)); rec.Code != http.StatusNoContent {
			t.Fatalf("visitante %d: estado = %d; todos compartian el balde del contenedor web", i+1, rec.Code)
		}
	}
	// Y cada visitante si tiene su tope.
	pedir(h, porSSR("/api/v1/public/pages/inicio", "203.0.113.1"))
	if rec := pedir(h, porSSR("/api/v1/public/pages/inicio", "203.0.113.1")); rec.Code != http.StatusTooManyRequests {
		t.Errorf("un visitante via SSR se salto su tope: %d", rec.Code)
	}
}

// Criterio 5: quien cambia su X-Forwarded-For en cada peticion sigue contando
// contra su IP real, la que agrego el edge al final.
func TestCambiarElForwardedForNoDaUnaIpNueva(t *testing.T) {
	h := cadena(New(3, time.Minute), New(3, time.Minute))
	for i := range 3 {
		pedir(h, porEdge("/api/v1/pages", fmt.Sprintf("10.0.0.%d, 203.0.113.7", i)))
	}
	if rec := pedir(h, porEdge("/api/v1/pages", "10.9.9.9, 203.0.113.7")); rec.Code != http.StatusTooManyRequests {
		t.Errorf("estado = %d; la primera entrada la elige el cliente", rec.Code)
	}
}

// Criterio 6: por el edge, las cabeceras del SSR sin el secreto no dan nada:
// ni el tope de otra IP ni saltarse el propio.
func TestImitarAlSSRSinElSecretoNoSirve(t *testing.T) {
	h := cadena(New(2, time.Minute), New(2, time.Minute))
	for i := range 2 {
		pedir(h, porEdge("/api/v1/pages", "203.0.113.7",
			httpx.CabeceraSecretoSSR, "adivinado", httpx.CabeceraIPDelSSR, fmt.Sprintf("198.51.100.%d", i)))
	}
	rec := pedir(h, porEdge("/api/v1/pages", "203.0.113.7",
		httpx.CabeceraSecretoSSR, "adivinado", httpx.CabeceraIPDelSSR, "198.51.100.99"))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("estado = %d; las cabeceras del SSR sin secreto le dieron IPs nuevas", rec.Code)
	}
	// Y la IP que dijo ser sigue intacta.
	if rec := pedir(h, porEdge("/api/v1/pages", "198.51.100.0")); rec.Code != http.StatusNoContent {
		t.Errorf("la IP imitada perdio su tope: %d", rec.Code)
	}
}

// Criterio 7: muchas IPs que no vuelven se liberan solas, sin nada que
// arrancar a mano.
func TestLasIpsQueNoVuelvenSeLiberan(t *testing.T) {
	rj := &reloj{t: time.Now()}
	l := conReloj(New(10, time.Minute), rj)

	for i := range 1000 {
		l.Tomar(strconv.Itoa(i))
	}
	if n := len(l.baldes); n != 1000 {
		t.Fatalf("baldes = %d", n)
	}

	rj.avanzar(time.Minute)
	l.Tomar("la-que-vuelve")

	if n := len(l.baldes); n != 1 {
		t.Errorf("baldes = %d; pasada la ventana tenian que liberarse", n)
	}
}

// Criterio 8: el healthcheck pega cada pocos segundos desde la misma IP. Un
// 429 ahi reiniciaria un proceso sano.
func TestSaludYDisponibilidadNuncaDan429(t *testing.T) {
	h := cadena(New(1, time.Minute), New(1, time.Minute))
	for _, ruta := range []string{"/api/v1/healthz", "/api/v1/readyz"} {
		for range 5 {
			r := httptest.NewRequest(http.MethodGet, ruta, nil)
			r.RemoteAddr = "127.0.0.1:9999"
			if rec := pedir(h, r); rec.Code != http.StatusNoContent {
				t.Fatalf("%s: estado = %d", ruta, rec.Code)
			}
		}
	}
}

// Los numeros de la HU. Una prueba que los fija obliga a que cambiarlos sea una
// decision y no un descuido.
func TestLosTopesSonLosDeLaHistoria(t *testing.T) {
	if TopeGeneral != 600 || TopeAuth != 30 || Ventana != time.Minute {
		t.Errorf("topes = %d/%d por %s", TopeGeneral, TopeAuth, Ventana)
	}
}
