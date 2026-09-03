package settings

import (
	"context"
	"errors"
	"net/url"

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
	actualizar(ctx context.Context, s Setting) (Setting, error)
}

type Service struct {
	repo repositorio
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
	if prob := validarValor(e.Value); prob != nil {
		return Setting{}, prob
	}

	creada, err := s.repo.crear(ctx, Setting{Key: e.Key, Value: e.Value, IsPublic: publico(e.IsPublic)})
	return creada, traducirEstado(err)
}

// Reemplazar exige la version que el cliente creia estar editando. Si en la
// base hay otra, alguien guardo en medio y la respuesta es 409, no un guardado
// que borra el trabajo ajeno en silencio.
func (s *Service) Reemplazar(ctx context.Context, key string, version int, m SettingModificacion) (Setting, error) {
	if prob := validarValor(m.Value); prob != nil {
		return Setting{}, prob
	}

	actualizada, err := s.repo.actualizar(ctx, Setting{
		Key:      key,
		Value:    m.Value,
		IsPublic: publico(m.IsPublic),
		Version:  version,
	})
	return actualizada, traducirEstado(err)
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
