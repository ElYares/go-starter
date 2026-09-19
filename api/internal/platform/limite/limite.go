// Package limite es el tope de peticiones por IP de toda la API (HU-010).
//
// No es el limite del login: aquel cuenta FALLOS por correo y por IP y vive en
// platform/auth. Este cuenta peticiones, cualquiera, y existe para que un solo
// cliente no pueda saturar el proceso.
//
// **Lo que esto NO resuelve**, igual que el de intentos (Decision 018): el
// contador vive en memoria, se pierde al reiniciar y no se comparte entre
// replicas. Con dos detras de un balanceador, el tope efectivo es el doble.
package limite

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Los topes de HU-010, por IP y por minuto. Seiscientos son diez por segundo:
// el dashboard y una landing con imagenes caben de sobra. `/auth` es login,
// refresh, logout y me; treinta dan para rehidratar la sesion en cada
// navegacion, y el login conserva ademas su limite de intentos fallidos.
const (
	TopeGeneral = 600
	TopeAuth    = 30
	Ventana     = time.Minute
)

// Limitador cuenta peticiones por clave en ventanas fijas.
//
// Ventana fija y no deslizante: la deslizante guarda un sello por peticion, y
// con un tope de seiscientos eso son seiscientos sellos por IP en memoria. La
// fija guarda un contador. Su defecto conocido —hasta el doble del tope en el
// borde entre dos ventanas— no importa para frenar una saturacion.
type Limitador struct {
	mu      sync.Mutex
	tope    int
	ventana time.Duration
	baldes  map[string]balde
	ahora   func() time.Time
	// ultimoBarrido es cuando Tomar barrio por ultima vez. Ver Tomar.
	ultimoBarrido time.Time
}

type balde struct {
	inicio time.Time
	n      int
}

func New(tope int, ventana time.Duration) *Limitador {
	return &Limitador{tope: tope, ventana: ventana, baldes: make(map[string]balde), ahora: time.Now}
}

// Tomar cuenta una peticion de la clave y dice si cabe. Si no, cuanto falta
// para que empiece la ventana siguiente.
//
// El barrido va aqui por la misma razon que en Intentos.Fallo: Tomar es lo
// unico que hace crecer el mapa, asi que barrer en el mismo sitio acota la
// memoria sin una goroutine que alguien tenga que acordarse de arrancar. Se
// barre como mucho una vez por ventana.
func (l *Limitador) Tomar(clave string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ahora := l.ahora()
	if ahora.Sub(l.ultimoBarrido) >= l.ventana {
		for k, b := range l.baldes {
			if ahora.Sub(b.inicio) >= l.ventana {
				delete(l.baldes, k)
			}
		}
		l.ultimoBarrido = ahora
	}

	// El balde vencido se reinicia aqui y no se deja al barrido: el barrido
	// corre una vez por ventana, y hasta que le toque una IP que ya cumplio su
	// minuto seguiria bloqueada.
	b, ok := l.baldes[clave]
	if !ok || ahora.Sub(b.inicio) >= l.ventana {
		b = balde{inicio: ahora}
	}
	if b.n >= l.tope {
		// Insistir no alarga la espera: la ventana es fija y termina a su hora,
		// cuente o no lo rechazado. Por eso no se escribe nada.
		return false, b.inicio.Add(l.ventana).Sub(ahora)
	}
	b.n++
	l.baldes[clave] = b
	return true, 0
}

// Middleware aplica los dos topes. Son independientes a proposito: agotar
// `/auth` —probar sesiones a fuerza bruta— no deja a esa IP sin ver la landing,
// y navegar mucho no le gasta el login.
//
// Salud y disponibilidad no cuentan: las consulta el healthcheck del
// contenedor desde la misma IP cada pocos segundos, y un 429 ahi haria que el
// orquestador reiniciara un proceso sano.
func Middleware(general, auth *Limitador) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ruta := r.URL.Path
			if ruta == "/api/v1/healthz" || ruta == "/api/v1/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			l := general
			if strings.HasPrefix(ruta, "/api/v1/auth/") {
				l = auth
			}

			if ok, espera := l.Tomar(httpx.ClientIP(r)); !ok {
				p := httpx.TooManyRequests("Demasiadas peticiones desde esta direccion. Espera un momento y vuelve a intentarlo")
				p.RetryAfter = max(espera, time.Second)
				httpx.WriteProblem(w, r, p)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
