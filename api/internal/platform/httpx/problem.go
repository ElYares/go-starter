// Package httpx es la forma en que este proyecto contesta HTTP: una sola manera
// de responder bien y una sola de responder mal.
//
// El molde completo esta en docs/04-reglas-de-crud.md. Aqui vive su mitad de
// codigo, y es normativa: un handler que escriba errores a mano se sale del
// contrato aunque compile.
package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/elyares/go-starter/api/internal/platform/observ"
)

// Codigos estables. Son lo que el cliente interpreta, asi que se agregan pero
// no se renombran: cambiar uno rompe a quien lo estaba mirando.
const (
	CodeValidationFailed = "VALIDATION_FAILED"
	CodeUnauthenticated  = "UNAUTHENTICATED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeTooManyRequests  = "TOO_MANY_REQUESTS"
	CodeUnsupportedMedia = "UNSUPPORTED_MEDIA_TYPE"
	CodePayloadTooLarge  = "PAYLOAD_TOO_LARGE"
	CodeInternal         = "INTERNAL_ERROR"
)

// FieldIssue es un problema de un campo concreto. `Code` es lo estable;
// `Message` es para humanos y puede cambiar sin romper nada.
type FieldIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem es RFC 7807 con dos campos propios: `code` y `traceId`.
type Problem struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	Code     string       `json:"code"`
	TraceID  string       `json:"traceId"`
	Errors   []FieldIssue `json:"errors,omitempty"`
}

// Error hace que un Problem se pueda devolver como error desde un service y
// llegar entero hasta el handler, sin traducciones intermedias que pierdan el
// codigo.
func (p *Problem) Error() string { return p.Title }

// New arma un Problem sin escribirlo. Util para devolverlo como error.
func New(status int, code, title, detail string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
	}
}

// WriteProblem es la unica salida de error de la API.
//
// El `traceId` y el `instance` se rellenan aqui, no en cada handler: si
// dependieran de que alguien se acuerde, la mitad de los errores saldrian sin
// forma de encontrarlos en el log.
func WriteProblem(w http.ResponseWriter, r *http.Request, p *Problem) {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Instance == "" {
		p.Instance = r.URL.Path
	}
	p.TraceID = observ.TraceIDFrom(r.Context())

	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.ErrorContext(r.Context(), "no se pudo escribir el problem+json",
			slog.String("error", err.Error()), slog.String("traceId", p.TraceID))
	}
}

// Atajos. Existen para que el codigo de negocio no invente titulos ni codigos
// distintos para la misma situacion.

func BadRequest(detail string, issues ...FieldIssue) *Problem {
	p := New(http.StatusBadRequest, CodeValidationFailed, "Validacion fallida", detail)
	p.Errors = issues
	return p
}

func Unauthorized() *Problem {
	// Mensaje generico a proposito: distinguir "no existe" de "contrasena mala"
	// convierte el login en un oraculo de que correos estan registrados.
	return New(http.StatusUnauthorized, CodeUnauthenticated, "No autenticado",
		"La peticion no trae una sesion valida")
}

func Forbidden() *Problem {
	return New(http.StatusForbidden, CodeForbidden, "Sin permiso",
		"La sesion no tiene permiso para esta operacion")
}

// NotFound se usa tambien cuando el recurso existe pero es de otro. Un 403
// confirmaria que existe, y con eso se enumeran ids ajenos.
func NotFound() *Problem {
	return New(http.StatusNotFound, CodeNotFound, "No encontrado",
		"El recurso no existe o no esta disponible para esta sesion")
}

func Conflict(detail string) *Problem {
	return New(http.StatusConflict, CodeConflict, "Conflicto", detail)
}

func TooManyRequests(detail string) *Problem {
	return New(http.StatusTooManyRequests, CodeTooManyRequests, "Demasiadas peticiones", detail)
}

// Internal nunca lleva detalle del error real: eso se queda en el log, junto al
// mismo traceId que ve el cliente.
func Internal() *Problem {
	return New(http.StatusInternalServerError, CodeInternal, "Error interno",
		"Algo fallo de nuestro lado. El traceId identifica esta peticion en el log")
}

// Problemas de parametro. Existen aqui, y no en cada modulo, porque el codigo
// que genera oapi-codegen deja sus tipos de error DENTRO del paquete de cada
// modulo: el `switch` que los distingue no se puede compartir, pero el texto de
// la respuesta si, y es lo que tiene que ser igual en toda la API.

func ParamRequired(name string) *Problem {
	return BadRequest(fmt.Sprintf("Falta el parametro %q", name), FieldIssue{
		Field: name, Code: "required", Message: "Este parametro es obligatorio",
	})
}

func ParamType(name string) *Problem {
	return BadRequest(
		fmt.Sprintf("El parametro %q no tiene el tipo que declara el contrato", name),
		FieldIssue{Field: name, Code: "type", Message: "El valor no tiene el tipo esperado"})
}

func ParamRepeated(name string) *Problem {
	return BadRequest(fmt.Sprintf("El parametro %q llego repetido", name), FieldIssue{
		Field: name, Code: "repeated", Message: "Este parametro solo acepta un valor",
	})
}

// ParamInvalid es el caso que no encaja en los anteriores. Sigue siendo culpa
// de la peticion, no nuestra: 400, no 500.
func ParamInvalid() *Problem {
	return BadRequest("Los parametros de la peticion no son validos")
}
