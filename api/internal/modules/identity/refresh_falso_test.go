package identity

import (
	"bytes"
	"context"
	"time"
)

// filaRefresh es una fila falsa de `refresh_tokens`. El compare-and-set de
// verdad vive en el WHERE del update y solo lo prueba la base
// (renovar_integracion_test.go); aqui se imita su efecto para probar lo que
// decide el service con cada resultado.
type filaRefresh struct {
	guardado RefreshGuardado
	hash     []byte
}

func (r *repoFalso) registrarRefresh(s SesionNueva) {
	r.refresh[s.ID] = &filaRefresh{
		guardado: RefreshGuardado{ID: s.ID, UserID: s.UserID, ExpiraEn: s.ExpiraEn},
		hash:     s.TokenHash,
	}
}

func (r *repoFalso) refreshPorHash(_ context.Context, hash []byte) (RefreshGuardado, error) {
	for _, f := range r.refresh {
		if bytes.Equal(f.hash, hash) {
			return f.guardado, nil
		}
	}
	return RefreshGuardado{}, errNoExiste
}

func (r *repoFalso) rotarRefresh(_ context.Context, anteriorID string, nueva SesionNueva) (bool, error) {
	r.vecesQueSeIntentoRotar++
	if r.antesDeRotar != nil {
		r.antesDeRotar()
	}
	f, ok := r.refresh[anteriorID]
	if !ok || f.guardado.ReemplazadoPor != nil || f.guardado.RevocadoEn != nil || !time.Now().Before(f.guardado.ExpiraEn) {
		return false, nil
	}
	id := nueva.ID
	f.guardado.ReemplazadoPor = &id
	r.registrarRefresh(nueva)
	return true, nil
}

func (r *repoFalso) revocarSesionesDe(_ context.Context, userID string) error {
	r.vecesQueSeRevocoTodo++
	ahora := time.Now()
	for _, f := range r.refresh {
		if f.guardado.UserID == userID && f.guardado.RevocadoEn == nil {
			f.guardado.RevocadoEn = &ahora
		}
	}
	return nil
}

func (r *repoFalso) revocarRefresh(_ context.Context, hash []byte) error {
	ahora := time.Now()
	for _, f := range r.refresh {
		if bytes.Equal(f.hash, hash) && f.guardado.RevocadoEn == nil {
			f.guardado.RevocadoEn = &ahora
		}
	}
	return nil
}

// vivas cuenta las sesiones de una persona que todavia podrian renovar.
func (r *repoFalso) vivas(userID string) int {
	n := 0
	for _, f := range r.refresh {
		g := f.guardado
		if g.UserID == userID && g.RevocadoEn == nil && g.ReemplazadoPor == nil {
			n++
		}
	}
	return n
}

func (r *repoFalso) filaDe(token string) *filaRefresh {
	h := hashDe(token)
	for _, f := range r.refresh {
		if bytes.Equal(f.hash, h) {
			return f
		}
	}
	return nil
}
