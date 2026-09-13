package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

func logDePrueba() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// capturaActor deja ver que actor quedo en el contexto, que es lo unico que
// hace este middleware.
func capturaActor(visto *rbac.Actor, hubo *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*visto, *hubo = rbac.ActorFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func resolverFijo(a rbac.Actor) ResolverActor {
	return func(context.Context, string) (rbac.Actor, error) { return a, nil }
}

func TestConUnaCookieValidaElActorQuedaEnElContexto(t *testing.T) {
	f := firmanteDePrueba(t)
	token, _ := f.Firmar("u-1", []string{"admin"})

	var visto rbac.Actor
	var hubo bool

	r := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	r.AddCookie(&http.Cookie{Name: CookieAT, Value: token})

	Sesion(f, resolverFijo(rbac.Actor{ID: "u-1", Permissions: []string{"settings.read"}}), logDePrueba())(
		capturaActor(&visto, &hubo)).ServeHTTP(httptest.NewRecorder(), r)

	if !hubo {
		t.Fatal("no quedo actor en el contexto")
	}
	if visto.ID != "u-1" || !visto.Can("settings.read") {
		t.Errorf("actor = %+v", visto)
	}
}

// El middleware NO rechaza nunca: quien decide si una ruta necesita sesion es
// el guard de la ruta. Si este rechazara, cerraria tambien la landing publica
// y el propio login.
func TestSinCookieSigueComoAnonimoYNoRechaza(t *testing.T) {
	var visto rbac.Actor
	var hubo bool

	rec := httptest.NewRecorder()
	Sesion(firmanteDePrueba(t), resolverFijo(rbac.Actor{ID: "u-1"}), logDePrueba())(
		capturaActor(&visto, &hubo)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/settings", nil))

	if hubo {
		t.Error("quedo un actor sin cookie")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("estado = %d; este middleware no rechaza", rec.Code)
	}
}

// Un token invalido acaba igual que la ausencia de token: anonimo. Para quien
// pide son lo mismo —no hay sesion— y la diferencia se ve en el log.
func TestConUnaCookieInvalidaSigueComoAnonimo(t *testing.T) {
	f := firmanteDePrueba(t)
	otro, _ := NewFirmante("otra-llave-distinta-de-la-de-arriba")
	ajeno, _ := otro.Firmar("u-1", nil)

	for _, valor := range []string{"cualquier-cosa", "", ajeno} {
		var visto rbac.Actor
		var hubo bool

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		r.AddCookie(&http.Cookie{Name: CookieAT, Value: valor})

		Sesion(f, resolverFijo(rbac.Actor{ID: "u-1"}), logDePrueba())(
			capturaActor(&visto, &hubo)).ServeHTTP(rec, r)

		if hubo {
			t.Errorf("con la cookie %q quedo un actor", valor)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("con la cookie %q el estado fue %d", valor, rec.Code)
		}
	}
}

// Si el resolver falla —la base caida, o el usuario borrado con la sesion
// abierta— la peticion sigue como anonima y el guard dara 401. Lo que NO puede
// pasar es que quede un actor a medias: seria una sesion sin permisos que se
// ve como "me expulsa solo" y no aparece en ningun sitio.
func TestSiElResolverFallaNoQuedaUnActorAMedias(t *testing.T) {
	f := firmanteDePrueba(t)
	token, _ := f.Firmar("u-1", []string{"admin"})

	var visto rbac.Actor
	var hubo bool

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	r.AddCookie(&http.Cookie{Name: CookieAT, Value: token})

	roto := func(context.Context, string) (rbac.Actor, error) {
		return rbac.Actor{}, errors.New("la base no contesta")
	}

	Sesion(f, roto, logDePrueba())(capturaActor(&visto, &hubo)).ServeHTTP(rec, r)

	if hubo {
		t.Error("quedo un actor con el resolver roto")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("estado = %d; este middleware no rechaza ni cuando falla el resolver", rec.Code)
	}
}

// El sub que se le pasa al resolver es el del token verificado, no uno que
// venga de otro sitio. Resolver por un id que el atacante controle seria
// suplantacion directa.
func TestElResolverRecibeElSujetoDelTokenVerificado(t *testing.T) {
	f := firmanteDePrueba(t)
	token, _ := f.Firmar("el-de-verdad", nil)

	var pedido string
	r := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	r.AddCookie(&http.Cookie{Name: CookieAT, Value: token})
	// Un id distinto en una cabecera, por si alguien lo lee de ahi algun dia.
	r.Header.Set("X-User-Id", "el-suplantado")

	espia := func(_ context.Context, id string) (rbac.Actor, error) {
		pedido = id
		return rbac.Actor{ID: id}, nil
	}

	var visto rbac.Actor
	var hubo bool
	Sesion(f, espia, logDePrueba())(capturaActor(&visto, &hubo)).
		ServeHTTP(httptest.NewRecorder(), r)

	if pedido != "el-de-verdad" {
		t.Errorf("el resolver recibio %q", pedido)
	}
}
