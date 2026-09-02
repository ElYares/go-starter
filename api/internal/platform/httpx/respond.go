package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// WriteJSON es la unica salida de exito. Escribe la cabecera despues de
// serializar no seria posible sin bufferizar, asi que un fallo de serializacion
// a media respuesta solo se puede registrar: el cliente ya recibio un 200.
// Por eso los DTO de salida no llevan tipos que no serialicen.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "respuesta a medias: fallo al serializar",
			slog.String("error", err.Error()),
			slog.String("traceId", observ.TraceIDFrom(r.Context())))
	}
}

// NoContent es la respuesta de un borrado y de las operaciones que no tienen
// nada util que devolver.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Created responde 201 con Location, que es lo que permite al cliente seguir
// trabajando sin adivinar la URL del recurso nuevo.
func Created(w http.ResponseWriter, r *http.Request, location string, body any) {
	w.Header().Set("Location", location)
	WriteJSON(w, r, http.StatusCreated, body)
}
