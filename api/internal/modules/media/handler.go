package media

import (
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// holguraMultipart es lo que ocupan las cabeceras del multipart alrededor del
// archivo. El cuerpo entero se corta en MaxBytes mas esto; el archivo, en
// MaxBytes exacto, dentro del service.
const holguraMultipart = 64 << 10

// SubirMedio lee el multipart parte por parte, sin ParseMultipartForm: ese
// guarda el archivo entero en memoria o en un temporal ANTES de dejar mirarlo,
// y aqui el tope y el hash se aplican mientras llega.
//
// Se lee hasta encontrar `file`. Un campo con otro nombre antes de el es 400;
// lo que venga despues no se lee.
func (m *Module) SubirMedio(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes+holguraMultipart)

	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteProblem(w, r, noEsMultipart())
		return
	}

	parte, err := mr.NextPart()
	switch {
	case errors.Is(err, io.EOF):
		httpx.WriteProblem(w, r, sinArchivo())
		return
	case err != nil:
		m.fallo(w, r, err)
		return
	case parte.FormName() != "file":
		httpx.WriteProblem(w, r, campoDesconocido(parte.FormName()))
		return
	}

	medio, creado, err := m.svc.Subir(r.Context(), parte.FileName(), parte)
	if err != nil {
		m.fallo(w, r, err)
		return
	}

	if creado {
		httpx.Created(w, r, "/api/v1/media/"+medio.Id.String(), medio)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, medio)
}

func (m *Module) LeerMedio(w http.ResponseWriter, r *http.Request, id MedioId) {
	medio, err := m.svc.Leer(r.Context(), id)
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, medio)
}

// LeerMedioPublico sirve los bytes. `nosniff` hace que el navegador use el tipo
// que se dedujo al subir y no adivine otro; la CSP impide que, si algun dia se
// cuela algo que no es imagen, se ejecute como pagina del mismo origen.
func (m *Module) LeerMedioPublico(w http.ResponseWriter, r *http.Request, id MedioId) {
	medio, rc, err := m.svc.Abrir(r.Context(), id)
	if err != nil {
		m.fallo(w, r, err)
		return
	}
	defer rc.Close()

	h := w.Header()
	h.Set("Content-Type", string(medio.Mime))
	h.Set("Content-Length", strconv.FormatInt(medio.SizeBytes, 10))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// El registro no cambia nunca: el mismo id son siempre los mismos bytes.
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		// La cabecera ya salio: solo queda registrarlo.
		slog.WarnContext(r.Context(), "media: se corto el envio de un archivo",
			slog.String("id", id.String()), slog.String("error", err.Error()))
	}
}

// fallo es la unica salida de error de los handlers del modulo.
func (m *Module) fallo(w http.ResponseWriter, r *http.Request, err error) {
	var (
		prob     *httpx.Problem
		maxBytes *http.MaxBytesError
	)
	switch {
	case errors.As(err, &prob):
		httpx.WriteProblem(w, r, prob)
		return
	case errors.Is(err, errNoExiste):
		httpx.WriteProblem(w, r, httpx.NotFound())
		return
	case errors.As(err, &maxBytes), errors.Is(err, errDemasiadoGrande):
		httpx.WriteProblem(w, r, demasiadoGrande())
		return
	case errors.Is(err, multipart.ErrMessageTooLarge):
		httpx.WriteProblem(w, r, demasiadoGrande())
		return
	}

	slog.ErrorContext(r.Context(), "media: fallo la operacion",
		slog.String("error", err.Error()),
		slog.String("traceId", observ.TraceIDFrom(r.Context())))
	httpx.WriteProblem(w, r, httpx.Internal())
}
