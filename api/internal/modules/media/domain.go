package media

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Los tipos de este modulo —Medio y el MIME— salen de api/openapi.yaml y viven
// en openapi_gen.go. Aqui queda lo que el contrato no puede expresar.

// MaxBytes es el tope de un archivo. Se aplica mientras se lee: comprobarlo al
// final significaria haber aceptado ya todo el cuerpo.
const MaxBytes = 5 << 20

// cabecera es lo que se lee antes de guardar nada, para decidir el tipo. Es lo
// que mira http.DetectContentType, que no usa mas.
const cabecera = 512

// permitidos son los tipos que se aceptan, con el formato que image.DecodeConfig
// tiene que reconocer en ellos. SVG no esta a proposito: es XML que puede
// llevar <script>, y servirlo desde el mismo origen es un XSS.
var permitidos = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpeg",
	"image/webp": "webp",
}

const maxNombre = 255

var (
	errNoExiste        = errors.New("media: el medio no existe")
	errDemasiadoGrande = errors.New("media: el archivo pasa del tope")
)

// llaveDe es la ruta en el almacenamiento. Sale del hash, asi que la misma
// imagen cae siempre en el mismo lugar. El primer byte hace de carpeta para no
// juntar miles de archivos en un solo directorio.
func llaveDe(sum []byte) string {
	h := hex.EncodeToString(sum)
	return h[:2] + "/" + h
}

func urlPublica(id uuid.UUID) string { return "/api/v1/public/media/" + id.String() }

// nombreOriginal recorta lo que manda el cliente. multipart ya se queda con la
// base del nombre; aqui se descarta lo vacio y lo que no es texto.
func nombreOriginal(n string) *string {
	if n == "" || !utf8.ValidString(n) {
		return nil
	}
	if utf8.RuneCountInString(n) > maxNombre {
		n = string([]rune(n)[:maxNombre])
	}
	return &n
}

func tipoNoPermitido(detectado string) *httpx.Problem {
	return httpx.BadRequest("El archivo no es una imagen permitida", httpx.FieldIssue{
		Field:   "file",
		Code:    "type",
		Message: fmt.Sprintf("Solo PNG, JPEG o WebP; el archivo es %s", detectado),
	})
}

func imagenRota() *httpx.Problem {
	return httpx.BadRequest("El archivo no es una imagen valida", httpx.FieldIssue{
		Field: "file", Code: "type", Message: "Los bytes no forman una imagen que se pueda leer",
	})
}

func sinArchivo() *httpx.Problem {
	return httpx.BadRequest("Falta el archivo", httpx.FieldIssue{
		Field: "file", Code: "required", Message: "Se sube en el campo file",
	})
}

func archivoVacio() *httpx.Problem {
	return httpx.BadRequest("El archivo esta vacio", httpx.FieldIssue{
		Field: "file", Code: "required", Message: "El archivo no tiene bytes",
	})
}

func campoDesconocido(nombre string) *httpx.Problem {
	return httpx.BadRequest("El cuerpo trae un campo que no se usa", httpx.FieldIssue{
		Field: nombre, Code: "unknown", Message: "Solo se acepta el campo file",
	})
}

func noEsMultipart() *httpx.Problem {
	return httpx.BadRequest("El cuerpo tiene que ser multipart/form-data", httpx.FieldIssue{
		Field: "file", Code: "required", Message: "Se sube como multipart/form-data, en el campo file",
	})
}

func demasiadoGrande() *httpx.Problem {
	return httpx.New(http.StatusRequestEntityTooLarge, httpx.CodePayloadTooLarge,
		"Archivo demasiado grande", fmt.Sprintf("El maximo es %d MB", MaxBytes>>20))
}
