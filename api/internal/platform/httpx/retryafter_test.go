package httpx

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// Un 429 sin Retry-After deja al cliente eligiendo un numero, y el numero que
// elige es "ya". Por eso la cabecera la pone WriteProblem y no el handler.
func TestUn429LlevaSuRetryAfterEnLaCabecera(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil),
		TooManyRequestsIn(90*time.Second))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("estado = %d", rec.Code)
	}

	segundos, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q, tiene que ser un entero de segundos", rec.Header().Get("Retry-After"))
	}
	if segundos != 90 {
		t.Errorf("Retry-After = %d, se esperaban 90", segundos)
	}
}

// Hacia arriba: redondear hacia abajo produce un "espera 0 segundos" que invita
// a reintentar de inmediato y recibir el mismo 429.
func TestElRetryAfterRedondeaHaciaArriba(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, httptest.NewRequest(http.MethodPost, "/x", nil),
		TooManyRequestsIn(1500*time.Millisecond))

	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Errorf("Retry-After = %q, se esperaba 2", got)
	}
}

// La espera es de transporte, no de contenido: va en la cabecera y NO en el
// cuerpo, que sigue siendo el mismo problem+json de siempre.
func TestLaEsperaNoSeCuelaEnElCuerpo(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, httptest.NewRequest(http.MethodPost, "/x", nil),
		TooManyRequestsIn(90*time.Second))

	if body := rec.Body.String(); contiene(body, "RetryAfter") || contiene(body, "retryAfter") {
		t.Errorf("la espera salio en el cuerpo: %s", body)
	}
}

// Los demas problemas no llevan la cabecera. Un Retry-After en un 400 le dice
// al cliente que reintente algo que va a fallar igual.
func TestLosProblemasSinEsperaNoLlevanLaCabecera(t *testing.T) {
	for _, p := range []*Problem{BadRequest("mal"), Unauthorized(), Forbidden(), NotFound(), Internal()} {
		rec := httptest.NewRecorder()
		WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/x", nil), p)

		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("%s salio con Retry-After %q", p.Code, got)
		}
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
