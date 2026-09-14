package content

import (
	"context"
	"errors"
	"net/url"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/paging"
)

// repositorio es la costura entre el service y Postgres, para que las pruebas
// del molde corran sin base de datos.
//
// Ningun metodo recibe al actor para filtrar por propiedad, y es deliberado:
// una pagina es del sitio, no de quien la creo. Lo que decide quien la toca es
// el permiso de la ruta. Ver docs/04-reglas-de-crud.md seccion 5 y la nota de
// la seccion 8 en este modulo.
type repositorio interface {
	pagina(ctx context.Context, p paging.Params) ([]PaginaResumen, int64, error)
	versiones(ctx context.Context, id uuid.UUID, p paging.Params) ([]VersionResumen, int64, error)
	obtener(ctx context.Context, id uuid.UUID) (Pagina, error)
	publica(ctx context.Context, slug string) (PaginaPublica, error)
	crear(ctx context.Context, b borrador) (Pagina, error)
	guardar(ctx context.Context, id uuid.UUID, version int, b borrador) (Pagina, error)
	publicar(ctx context.Context, id, versionID uuid.UUID) (Pagina, error)
	borrar(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo repositorio
	cat  *catalogo
}

func (s *Service) Pagina(ctx context.Context, q url.Values) (PaginasPage, error) {
	p, prob := listadoDePaginas.Parse(q)
	if prob != nil {
		return PaginasPage{}, prob
	}

	items, total, err := s.repo.pagina(ctx, p)
	if err != nil {
		return PaginasPage{}, err
	}

	pagina := paging.NewPage(items, p, total)
	return PaginasPage{Content: pagina.Content, Page: PageMeta(pagina.Page)}, nil
}

func (s *Service) Versiones(ctx context.Context, id uuid.UUID, q url.Values) (VersionesPage, error) {
	p, prob := listadoDeVersiones.Parse(q)
	if prob != nil {
		return VersionesPage{}, prob
	}

	items, total, err := s.repo.versiones(ctx, id, p)
	if err != nil {
		return VersionesPage{}, traducirEstado(err)
	}

	pagina := paging.NewPage(items, p, total)
	return VersionesPage{Content: pagina.Content, Page: PageMeta(pagina.Page)}, nil
}

func (s *Service) Leer(ctx context.Context, id uuid.UUID) (Pagina, error) {
	p, err := s.repo.obtener(ctx, id)
	return p, traducirEstado(err)
}

// Publica responde 404 tanto si el slug no existe como si la pagina nunca se
// publico. El publico no tiene por que saber que hay un borrador.
func (s *Service) Publica(ctx context.Context, slug string) (PaginaPublica, error) {
	p, err := s.repo.publica(ctx, slug)
	return p, traducirEstado(err)
}

// Crear valida TODO antes de escribir. Un bloque invalido no deja una pagina
// creada sin su version, ni una version con la mitad de los bloques.
func (s *Service) Crear(ctx context.Context, n PaginaNueva) (Pagina, error) {
	b := borradorDeAlta(n)
	if prob := b.validar(s.cat); prob != nil {
		return Pagina{}, prob
	}

	creada, err := s.repo.crear(ctx, b)
	return creada, traducirEstado(err)
}

// Guardar crea una version nueva y no toca la publicada. La validacion ocurre
// aqui, al guardar, y no al publicar: si no, el editor deja construir media
// hora algo que va a rebotar.
func (s *Service) Guardar(ctx context.Context, id uuid.UUID, version int, m PaginaModificacion) (Pagina, error) {
	b := borradorDeGuardado(m)
	if prob := b.validar(s.cat); prob != nil {
		return Pagina{}, prob
	}

	guardada, err := s.repo.guardar(ctx, id, version, b)
	return guardada, traducirEstado(err)
}

// Publicar no revalida los bloques: la version ya paso la validacion cuando se
// guardo, y las versiones no se modifican.
func (s *Service) Publicar(ctx context.Context, id uuid.UUID, p Publicacion) (Pagina, error) {
	publicada, err := s.repo.publicar(ctx, id, p.VersionId)
	return publicada, traducirEstado(err)
}

func (s *Service) Borrar(ctx context.Context, id uuid.UUID) error {
	return traducirEstado(s.repo.borrar(ctx, id))
}

// traducirEstado es el unico lugar donde un sentinela del repositorio se
// convierte en un codigo HTTP.
func traducirEstado(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNoExiste):
		return httpx.NotFound()
	case errors.Is(err, errSlugOcupado):
		return httpx.Conflict("Ya existe una pagina con ese slug")
	case errors.Is(err, errVersion):
		return httpx.Conflict(
			"La pagina cambio despues de que la leiste. Vuelve a cargarla y repite el cambio")
	case errors.Is(err, errVersionAjena):
		// 400 y no 404: la pagina existe y la ruta es correcta; lo que esta mal
		// es un campo del cuerpo, y el editor tiene que saber cual.
		return httpx.BadRequest("La version no es de esta pagina", httpx.FieldIssue{
			Field: "versionId", Code: "not_found",
			Message: "Esta pagina no tiene una version con ese id",
		})
	default:
		return err
	}
}
