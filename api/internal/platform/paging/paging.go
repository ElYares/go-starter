// Package paging es la unica forma de listar de este proyecto: un envoltorio
// fijo, un tope de tamano que no se puede pedir mas alto, y listas blancas de
// orden y filtro declaradas por recurso.
//
// El molde completo esta en docs/04-reglas-de-crud.md seccion 3. Aqui vive su
// mitad de codigo, y es normativa: un endpoint de coleccion que arme su propio
// `order by` se sale del contrato aunque funcione.
//
// La invariante que sostiene todo lo demas: ningun fragmento de SQL viaja
// desde el cliente. El cliente manda NOMBRES DEL CONTRATO; las columnas salen
// del Spec, que es codigo. Un nombre que no esta en el Spec es un 400, no una
// consulta rara.
package paging

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
)

const (
	// DefaultSize es lo que se devuelve cuando el cliente no pide tamano.
	DefaultSize = 20

	// MaxSize es un tope duro, no una sugerencia. Sin el, `?size=1000000` es
	// una negacion de servicio de una linea. Pedir mas no es un error: se
	// devuelve el maximo y `page.size` reporta el tamano EFECTIVO.
	MaxSize = 100

	paramPage = "page"
	paramSize = "size"
	paramSort = "sort"
)

// Kind es el tipo del valor de un filtro. Existe para convertir en el borde:
// mandar el texto "true" a una columna boolean deja que Postgres decida, y lo
// que decide con "tal vez" es un 500.
type Kind int

const (
	Text Kind = iota
	Bool
)

type campo struct {
	columna string
	kind    Kind
}

// Order es un criterio de orden ya resuelto a columna real.
type Order struct {
	Column string
	Desc   bool
}

// Spec es la lista blanca de un recurso: que se puede ordenar, que se puede
// filtrar y con que se desempata. Se declara junto al repositorio del recurso,
// no en un mapa global: un mapa global obliga a abrir un archivo compartido
// para agregar un campo, y ahi es donde se cuela el campo de otro recurso.
type Spec struct {
	sort     map[string]string
	filter   map[string]campo
	def      []Order
	tiebreak string
}

// NewSpec exige el desempate como argumento, no como campo opcional.
//
// Sin desempate, dos filas con el mismo valor en el campo de orden pueden
// repetirse o perderse entre paginas: Postgres no promete un orden estable
// para las filas empatadas, y con OFFSET cada pagina es una consulta distinta.
// Es el bug que nadie reporta porque parece "un registro que se perdio".
func NewSpec(tiebreak string) *Spec {
	if tiebreak == "" {
		panic("paging: el desempate no puede estar vacio")
	}
	return &Spec{
		sort:     map[string]string{},
		filter:   map[string]campo{},
		tiebreak: tiebreak,
	}
}

// SortBy abre un campo al orden. `field` es el nombre del contrato (camelCase,
// el que ve el cliente); `column` es la columna real.
func (s *Spec) SortBy(field, column string) *Spec {
	s.sort[field] = column
	return s
}

// FilterBy abre un campo al filtro, con su tipo.
func (s *Spec) FilterBy(field, column string, k Kind) *Spec {
	s.filter[field] = campo{columna: column, kind: k}
	return s
}

// DefaultOrder es el orden cuando el cliente no pide ninguno. Si no se declara,
// se ordena solo por el desempate.
func (s *Spec) DefaultOrder(o ...Order) *Spec {
	s.def = o
	return s
}

// Params es lo que pidio el cliente, ya saneado.
//
// Las columnas y los valores quedan en campos NO exportados a proposito: una
// vez que Parse devolvio esto, nadie de fuera puede meter una columna que no
// paso por la lista blanca.
type Params struct {
	Number int
	Size   int

	orders   []Order
	filters  []filtro
	tiebreak string
}

type filtro struct {
	columna string
	valor   any
}

// Parse lee la query y devuelve o los parametros saneados, o un Problem listo
// para escribir. Nunca devuelve las dos cosas.
func (s *Spec) Parse(q url.Values) (Params, *httpx.Problem) {
	p := Params{Size: DefaultSize, tiebreak: s.tiebreak}

	if raw := q.Get(paramPage); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return Params{}, problema(paramPage, "type", "Tiene que ser un entero mayor o igual a 0")
		}
		p.Number = n
	}

	if raw := q.Get(paramSize); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return Params{}, problema(paramSize, "type", "Tiene que ser un entero mayor o igual a 1")
		}
		p.Size = min(n, MaxSize)
	}

	for _, raw := range q[paramSort] {
		o, prob := s.orden(raw)
		if prob != nil {
			return Params{}, prob
		}
		p.orders = append(p.orders, o)
	}
	if len(p.orders) == 0 {
		p.orders = slices.Clone(s.def)
	}

	// Ordenado para que el mensaje de error sea el mismo en cada ejecucion: el
	// recorrido de un mapa en Go no tiene orden, y una prueba que afirme sobre
	// un mensaje no determinista falla una vez de cada tres.
	nombres := slices.Sorted(maps.Keys(q))
	for _, nombre := range nombres {
		if nombre == paramPage || nombre == paramSize || nombre == paramSort {
			continue
		}
		f, ok := s.filter[nombre]
		if !ok {
			return Params{}, problema(nombre, "unknown",
				"Este recurso no se filtra por aqui. Campos validos: "+listar(slices.Sorted(maps.Keys(s.filter))))
		}
		valores := q[nombre]
		if len(valores) > 1 {
			return Params{}, problema(nombre, "repeated",
				"Este filtro solo acepta un valor; llegaron "+strconv.Itoa(len(valores)))
		}
		valor, prob := convertir(nombre, valores[0], f.kind)
		if prob != nil {
			return Params{}, prob
		}
		p.filters = append(p.filters, filtro{columna: f.columna, valor: valor})
	}

	return p, nil
}

func (s *Spec) orden(raw string) (Order, *httpx.Problem) {
	field, dir, _ := strings.Cut(raw, ",")
	field = strings.TrimSpace(field)

	columna, ok := s.sort[field]
	if !ok {
		return Order{}, problema(paramSort, "unknown",
			fmt.Sprintf("No se puede ordenar por %q. Campos validos: %s", field, listar(slices.Sorted(maps.Keys(s.sort)))))
	}

	switch strings.ToLower(strings.TrimSpace(dir)) {
	case "", "asc":
		return Order{Column: columna}, nil
	case "desc":
		return Order{Column: columna, Desc: true}, nil
	default:
		return Order{}, problema(paramSort, "direction", `La direccion solo puede ser "asc" o "desc"`)
	}
}

func convertir(nombre, raw string, k Kind) (any, *httpx.Problem) {
	switch k {
	case Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, problema(nombre, "type", `Tiene que ser "true" o "false"`)
		}
		return b, nil
	default:
		return raw, nil
	}
}

// OrderBy arma el `order by` completo, desempate incluido.
//
// Todas las cadenas que salen de aqui vienen del Spec, nunca de la peticion.
func (p Params) OrderBy() string {
	partes := make([]string, 0, len(p.orders)+1)
	desempatado := false

	for _, o := range p.orders {
		dir := "asc"
		if o.Desc {
			dir = "desc"
		}
		partes = append(partes, o.Column+" "+dir)
		if o.Column == p.tiebreak {
			desempatado = true
		}
	}

	// Repetir la columna de desempate cuando ya se ordeno por ella no aporta
	// nada y ensucia el plan.
	if !desempatado {
		partes = append(partes, p.tiebreak+" asc")
	}

	return "order by " + strings.Join(partes, ", ")
}

// Where devuelve la clausula y sus argumentos, con los marcadores numerados a
// partir de `desde`. Los valores viajan como parametros: nunca se interpolan.
func (p Params) Where(desde int) (string, []any) {
	if len(p.filters) == 0 {
		return "", nil
	}

	partes := make([]string, 0, len(p.filters))
	args := make([]any, 0, len(p.filters))
	for i, f := range p.filters {
		partes = append(partes, fmt.Sprintf("%s = $%d", f.columna, desde+i))
		args = append(args, f.valor)
	}

	return "where " + strings.Join(partes, " and "), args
}

func (p Params) Limit() int  { return p.Size }
func (p Params) Offset() int { return p.Number * p.Size }

// Meta son los metadatos de la pagina. `Size` es el tamano EFECTIVO: si el
// cliente pidio un millon, aqui dice 100.
type Meta struct {
	Number        int   `json:"number"`
	Size          int   `json:"size"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
}

// Page es el envoltorio de toda coleccion de la API. Nunca un array pelado en
// la raiz: agregar metadatos despues rompe a todos los clientes a la vez.
type Page[T any] struct {
	Content []T  `json:"content"`
	Page    Meta `json:"page"`
}

// NewPage arma la respuesta. Una lista vacia sale como `[]`, jamas como `null`:
// un cliente que hace `data.content.map(...)` revienta con null y no con [].
func NewPage[T any](items []T, p Params, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}

	paginas := 0
	if total > 0 && p.Size > 0 {
		paginas = int((total + int64(p.Size) - 1) / int64(p.Size))
	}

	return Page[T]{
		Content: items,
		Page: Meta{
			Number:        p.Number,
			Size:          p.Size,
			TotalElements: total,
			TotalPages:    paginas,
		},
	}
}

func problema(campo, code, mensaje string) *httpx.Problem {
	return httpx.BadRequest(
		fmt.Sprintf("El parametro %q no es valido", campo),
		httpx.FieldIssue{Field: campo, Code: code, Message: mensaje})
}

func listar(campos []string) string {
	if len(campos) == 0 {
		return "(ninguno)"
	}
	return strings.Join(campos, ", ")
}
