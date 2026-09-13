package identity

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/auth"
	"github.com/elyares/go-starter/api/internal/platform/ids"
)

// Lo que solo existe en la base: el compare-and-set del refresh vive en el
// WHERE del update, y la carrera que cierra solo aparece con transacciones de
// verdad. El doble de las pruebas sin base diria que si sin comprobar nada.

func sesionDePrueba(t *testing.T, userID string, caduca time.Time) SesionNueva {
	t.Helper()
	id, err := ids.NewString()
	if err != nil {
		t.Fatalf("ids: %v", err)
	}
	rt, err := auth.NuevoRefreshToken()
	if err != nil {
		t.Fatalf("NuevoRefreshToken: %v", err)
	}
	return SesionNueva{ID: id, UserID: userID, TokenHash: auth.HashDeRefresh(rt), ExpiraEn: caduca}
}

func filasDeRefresh(t *testing.T, r *Repo, userID string) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`select count(*) from refresh_tokens where user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("contando: %v", err)
	}
	return n
}

// A1 en la base: cinco rotaciones simultaneas del mismo token. Gana UNA, y las
// perdedoras no dejan filas: su insert se va con el rollback. Sin el
// `replaced_by is null` del WHERE, las cinco rotarian y quedarian cinco
// sucesoras validas de una sola sesion.
func TestIntegracionDeCincoRotacionesSimultaneasGanaUna(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolAdmin)
	ctx := context.Background()

	original := sesionDePrueba(t, u.ID, time.Now().Add(time.Hour))
	if err := r.guardarRefresh(ctx, original); err != nil {
		t.Fatalf("guardarRefresh: %v", err)
	}

	const n = 5
	var (
		largada sync.WaitGroup
		fin     sync.WaitGroup
		mu      sync.Mutex
		ganadas int
		errores []error
	)
	largada.Add(1)
	for i := 0; i < n; i++ {
		fin.Add(1)
		sucesora := sesionDePrueba(t, u.ID, time.Now().Add(time.Hour))
		go func() {
			defer fin.Done()
			largada.Wait()
			ok, err := r.rotarRefresh(ctx, original.ID, sucesora)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errores = append(errores, err)
			}
			if ok {
				ganadas++
			}
		}()
	}
	largada.Done()
	fin.Wait()

	if len(errores) > 0 {
		t.Fatalf("rotaciones con error: %v", errores)
	}
	if ganadas != 1 {
		t.Errorf("rotaciones ganadas = %d; tiene que ganar exactamente una", ganadas)
	}
	if got := filasDeRefresh(t, r, u.ID); got != 2 {
		t.Errorf("filas = %d; la original y UNA sucesora, las perdedoras no dejan rastro", got)
	}
}

// La fila anterior queda apuntando a la sucesora, que es lo que despues delata
// el reuso. Y se lee de vuelta con refreshPorHash, que es como la lee el service.
func TestIntegracionRotarEncadenaHaciaLaSucesora(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolAdmin)
	ctx := context.Background()

	original := sesionDePrueba(t, u.ID, time.Now().Add(time.Hour))
	sucesora := sesionDePrueba(t, u.ID, time.Now().Add(time.Hour))
	if err := r.guardarRefresh(ctx, original); err != nil {
		t.Fatalf("guardarRefresh: %v", err)
	}
	if ok, err := r.rotarRefresh(ctx, original.ID, sucesora); err != nil || !ok {
		t.Fatalf("rotarRefresh = %v, %v", ok, err)
	}

	g, err := r.refreshPorHash(ctx, original.TokenHash)
	if err != nil {
		t.Fatalf("refreshPorHash: %v", err)
	}
	if g.ReemplazadoPor == nil || *g.ReemplazadoPor != sucesora.ID {
		t.Errorf("replaced_by = %v, se esperaba %s", g.ReemplazadoPor, sucesora.ID)
	}
	if g.UserID != u.ID || g.RevocadoEn != nil {
		t.Errorf("fila leida mal: %+v", g)
	}
}

// Un logout que llega antes le gana al refresh: renovar una sesion cerrada no
// la resucita.
func TestIntegracionUnLogoutLeGanaAlRefresh(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolAdmin)
	ctx := context.Background()

	original := sesionDePrueba(t, u.ID, time.Now().Add(time.Hour))
	if err := r.guardarRefresh(ctx, original); err != nil {
		t.Fatalf("guardarRefresh: %v", err)
	}
	if err := r.revocarRefresh(ctx, original.TokenHash); err != nil {
		t.Fatalf("revocarRefresh: %v", err)
	}

	ok, err := r.rotarRefresh(ctx, original.ID, sesionDePrueba(t, u.ID, time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("rotarRefresh: %v", err)
	}
	if ok {
		t.Error("se roto una sesion revocada")
	}
	if got := filasDeRefresh(t, r, u.ID); got != 1 {
		t.Errorf("filas = %d; la rotacion fallida no puede dejar la sucesora", got)
	}
}

// El reloj que cuenta es el de la base: un token caducado no rota aunque el
// service lo hubiera dado por bueno.
func TestIntegracionUnTokenCaducadoNoRota(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolAdmin)
	ctx := context.Background()

	caducado := sesionDePrueba(t, u.ID, time.Now().Add(-time.Minute))
	if err := r.guardarRefresh(ctx, caducado); err != nil {
		t.Fatalf("guardarRefresh: %v", err)
	}

	ok, err := r.rotarRefresh(ctx, caducado.ID, sesionDePrueba(t, u.ID, time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("rotarRefresh: %v", err)
	}
	if ok {
		t.Error("se roto un token caducado")
	}
}

// La revocacion general es de UNA persona. Un WHERE que se olvide del user_id
// cerraria la sesion de toda la instalacion con el primer reuso.
func TestIntegracionRevocarLasSesionesDeUnoNoTocaAOtro(t *testing.T) {
	s, r := servicioReal(t)
	ana := crearDePrueba(t, s, r, RolAdmin)
	beto := crearDePrueba(t, s, r, RolAdmin)
	ctx := context.Background()

	deAna := sesionDePrueba(t, ana.ID, time.Now().Add(time.Hour))
	deBeto := sesionDePrueba(t, beto.ID, time.Now().Add(time.Hour))
	for _, x := range []SesionNueva{deAna, deBeto} {
		if err := r.guardarRefresh(ctx, x); err != nil {
			t.Fatalf("guardarRefresh: %v", err)
		}
	}

	if err := r.revocarSesionesDe(ctx, ana.ID); err != nil {
		t.Fatalf("revocarSesionesDe: %v", err)
	}

	ga, _ := r.refreshPorHash(ctx, deAna.TokenHash)
	gb, _ := r.refreshPorHash(ctx, deBeto.TokenHash)
	if ga.RevocadoEn == nil {
		t.Error("la sesion de ana no se revoco")
	}
	if gb.RevocadoEn != nil {
		t.Error("revocar las sesiones de ana cerro tambien la de beto")
	}
}
