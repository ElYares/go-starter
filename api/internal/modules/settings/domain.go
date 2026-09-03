package settings

import (
	"encoding/json"
	"time"
)

// Setting es una clave de configuracion. `Value` viaja como JSON crudo: el
// esquema por clave se valida en la fase 4, y hasta entonces no tiene sentido
// inventarle un tipo de Go que no representa nada.
type Setting struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	IsPublic  bool            `json:"isPublic"`
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
}
