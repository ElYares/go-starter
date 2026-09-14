package content

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

// Los tipos de este modulo —Pagina, PaginaNueva, Bloque...— salen de
// api/openapi.yaml y viven en openapi_gen.go. Aqui queda lo que el contrato no
// puede expresar: reglas con mensajes propios.

// Un slug es la direccion de la pagina: aparece en la URL publica. El patron es
// el mismo que declara el contrato y el CHECK de la migracion; si uno cambia,
// cambian los tres.
var slugValido = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxBloques = 100

// borrador es lo que se guarda: la direccion y el contenido de una version. Es
// la misma forma para crear y para guardar, porque las dos operaciones escriben
// una version completa.
type borrador struct {
	Slug           string
	Title          string
	SeoTitle       *string
	SeoDescription *string
	Blocks         []Bloque
	Note           *string
}

func borradorDeAlta(n PaginaNueva) borrador {
	return borrador{Slug: n.Slug, Title: n.Title, SeoTitle: n.SeoTitle,
		SeoDescription: n.SeoDescription, Blocks: n.Blocks, Note: n.Note}
}

func borradorDeGuardado(m PaginaModificacion) borrador {
	return borrador{Slug: m.Slug, Title: m.Title, SeoTitle: m.SeoTitle,
		SeoDescription: m.SeoDescription, Blocks: m.Blocks, Note: m.Note}
}

// validar junta en UN 400 los problemas de la pagina y los de cada bloque. Si
// no escribe nada hasta que todo cuadra, un guardado invalido no deja una
// version a medias.
func (b borrador) validar(cat *catalogo) *httpx.Problem {
	var issues []httpx.FieldIssue

	switch {
	case b.Slug == "":
		issues = append(issues, obligatorio("slug"))
	case utf8.RuneCountInString(b.Slug) > 80:
		issues = append(issues, maximo("slug", 80))
	case !slugValido.MatchString(b.Slug):
		issues = append(issues, httpx.FieldIssue{
			Field: "slug", Code: "format", Message: "Solo minusculas, numeros y guiones entre ellos",
		})
	}

	if b.Title == "" {
		issues = append(issues, obligatorio("title"))
	} else if utf8.RuneCountInString(b.Title) > 120 {
		issues = append(issues, maximo("title", 120))
	}

	issues = append(issues, opcionalHasta("seoTitle", b.SeoTitle, 120)...)
	issues = append(issues, opcionalHasta("seoDescription", b.SeoDescription, 300)...)
	issues = append(issues, opcionalHasta("note", b.Note, 280)...)

	switch {
	case b.Blocks == nil:
		// Ausente o null. Una pagina sin bloques se manda como `[]`: vaciar la
		// pagina tiene que ser una decision, no un campo olvidado.
		issues = append(issues, obligatorio("blocks"))
	case len(b.Blocks) > maxBloques:
		issues = append(issues, httpx.FieldIssue{
			Field: "blocks", Code: "max", Message: fmt.Sprintf("El maximo es %d bloques", maxBloques),
		})
	default:
		issues = append(issues, cat.validar(b.Blocks)...)
	}

	if len(issues) == 0 {
		return nil
	}
	return httpx.BadRequest(fmt.Sprintf("La pagina tiene %d campo(s) invalido(s)", len(issues)), issues...)
}

func maximo(campo string, n int) httpx.FieldIssue {
	return httpx.FieldIssue{Field: campo, Code: "max", Message: fmt.Sprintf("El maximo es %d caracteres", n)}
}

func opcionalHasta(campo string, v *string, n int) []httpx.FieldIssue {
	if v != nil && utf8.RuneCountInString(*v) > n {
		return []httpx.FieldIssue{maximo(campo, n)}
	}
	return nil
}
