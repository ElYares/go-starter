package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/esquema"
	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// repositorio es la costura entre el service y Postgres. Existe para que las
// pruebas del molde —el tope de size, el sort invalido, el 409— corran sin una
// base de datos, que es lo unico que hay en CI.
type repositorio interface {
	listar(ctx context.Context, soloPublicas bool) ([]Setting, error)
	pagina(ctx context.Context, p paging.Params) ([]Setting, int64, error)
	obtener(ctx context.Context, key string) (Setting, error)
	crear(ctx context.Context, s Setting) (Setting, error)
	actualizar(ctx context.Context, s Setting, publica *bool) (Setting, error)
}

type Service struct {
	repo     repositorio
	esquemas map[string]*esquema.Esquema
	medios   Medios
}

func (s *Service) Publicas(ctx context.Context) ([]Setting, error) {
	return s.repo.listar(ctx, true)
}

func (s *Service) Todas(ctx context.Context) ([]Setting, error) {
	return s.repo.listar(ctx, false)
}

// Pagina valida la query contra la lista blanca ANTES de tocar la base. Un sort
// invalido no llega nunca a ser SQL, y por eso es un 400 y no un 500.
func (s *Service) Pagina(ctx context.Context, q url.Values) (SettingsPage, error) {
	p, prob := listado.Parse(q)
	if prob != nil {
		return SettingsPage{}, prob
	}

	items, total, err := s.repo.pagina(ctx, p)
	if err != nil {
		return SettingsPage{}, err
	}

	pagina := paging.NewPage(items, p, total)

	// La conversion de paging.Meta a PageMeta es lo que ata el molde al
	// contrato, y lo hace EN TIEMPO DE COMPILACION: Go solo permite convertir
	// entre structs con los mismos campos y tipos. Si el contrato le agrega un
	// campo a `page`, o le cambia el tipo a uno, esta linea deja de compilar.
	return SettingsPage{Content: pagina.Content, Page: PageMeta(pagina.Page)}, nil
}

func (s *Service) Leer(ctx context.Context, key string) (Setting, error) {
	item, err := s.repo.obtener(ctx, key)
	return item, traducirEstado(err)
}

func (s *Service) Crear(ctx context.Context, e SettingNuevo) (Setting, error) {
	if prob := validarClave(e.Key); prob != nil {
		return Setting{}, prob
	}
	if err := s.validar(ctx, e.Key, e.Value); err != nil {
		return Setting{}, err
	}

	creada, err := s.repo.crear(ctx, Setting{Key: e.Key, Value: e.Value, IsPublic: publico(e.IsPublic)})
	return creada, traducirEstado(err)
}

// Reemplazar exige la version que el cliente creia estar editando. Si en la
// base hay otra, alguien guardo en medio y la respuesta es 409, no un guardado
// que borra el trabajo ajeno en silencio.
func (s *Service) Reemplazar(ctx context.Context, key string, version int, m SettingModificacion) (Setting, error) {
	if err := s.validar(ctx, key, m.Value); err != nil {
		return Setting{}, err
	}

	// isPublic ausente conserva la visibilidad. Si fuera `false`, un formulario
	// que solo edita el valor esconderia la clave de la landing al guardar.
	actualizada, err := s.repo.actualizar(ctx, Setting{
		Key:     key,
		Value:   m.Value,
		Version: version,
	}, m.IsPublic)
	return actualizada, traducirEstado(err)
}

// validar comprueba el valor contra el esquema de su clave y, si cumple, que
// cada imagen que nombra exista. Todo antes de tocar la base: un valor invalido
// no llega a escribirse ni a subir la version.
func (s *Service) validar(ctx context.Context, key string, valor any) error {
	if prob := validarValor(valor); prob != nil {
		return prob
	}
	e, ok := s.esquemas[key]
	if !ok {
		return claveSinEsquema(key, s.esquemas)
	}

	issues := e.Validar(valor, "value")
	if len(issues) == 0 && s.medios != nil {
		for _, c := range e.ConFormato(valor, formatoMedio, "value") {
			// El esquema ya exigio que fuera un uuid.
			existe, err := s.medios.Existe(ctx, uuid.MustParse(c.Valor))
			if err != nil {
				return err
			}
			if !existe {
				issues = append(issues, httpx.FieldIssue{
					Field: c.Ruta, Code: "unknown", Message: "No existe una imagen con ese id",
				})
			}
		}
	}

	if len(issues) > 0 {
		return httpx.BadRequest(fmt.Sprintf("El valor tiene %d campo(s) invalido(s)", len(issues)), issues...)
	}
	return nil
}

// traducirEstado es el unico lugar donde un sentinela del repositorio se
// convierte en un codigo HTTP. Concentrarlo aqui evita que dos endpoints
// contesten distinto a la misma situacion.
func traducirEstado(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNoExiste):
		return httpx.NotFound()
	case errors.Is(err, errYaExiste):
		return httpx.Conflict("Ya existe una clave con ese nombre")
	case errors.Is(err, errVersion):
		return httpx.Conflict(
			"La configuracion cambio despues de que la leiste. Vuelve a cargarla y repite el cambio")
	default:
		return err
	}
}
