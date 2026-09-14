package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/paging"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Estas pruebas son lo unico que ejecuta el SQL de verdad: el LATERAL del
// borrador, la llave compuesta de la version publicada, el cascade y el
// bloqueo que ordena dos guardados a la vez. Se saltan solas sin DATABASE_URL.
//
//	docker exec -w /workspace devherd-go-starter-<hash>-api-1 \
//	    go test ./internal/modules/content/ -run Integracion -v
func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("sin DATABASE_URL: la prueba de integracion necesita una base real")
	}

	ctx := context.Background()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectando: %v", err)
	}
	if err := p.Ping(ctx); err != nil {
		t.Fatalf("la base no responde: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

const idDeLuis = "8c2d4e6f-1a3b-4c5d-9e7f-0a1b2c3d4e5f"

func conActor(id string) context.Context {
	return rbac.WithActor(context.Background(), rbac.Actor{ID: id})
}

// paginaDePrueba crea una pagina con un slug que no choca con otra ejecucion,
// y la borra al terminar.
func paginaDePrueba(t *testing.T, r *Repo, titulo string) Pagina {
	t.Helper()
	b := borrador{
		Slug:   fmt.Sprintf("prueba-%d", time.Now().UnixNano()),
		Title:  titulo,
		Blocks: []Bloque{{Id: "b1", Type: "texto", Props: map[string]interface{}{"body": titulo}}},
	}
	p, err := r.crear(conActor(idDeAna), b)
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(context.Background(), `delete from pages where id = $1`, p.Id) })
	return p
}

func guardado(p Pagina, titulo string) borrador {
	return borrador{
		Slug:   p.Slug,
		Title:  titulo,
		Blocks: []Bloque{{Id: "b1", Type: "texto", Props: map[string]interface{}{"body": titulo}}},
	}
}

func contarVersiones(t *testing.T, r *Repo, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), `select count(*) from page_versions where page_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Criterio 1 de HU-009: las tablas y sus dos unicidades existen.
func TestIntegracionLaMigracionCreaLasTablasConSusUnicidades(t *testing.T) {
	r := &Repo{pool: pool(t)}

	restricciones := map[string]string{
		"pages_slug_key":             "u",
		"page_versions_numero_key":   "u",
		"pages_version_publicada_fk": "f",
	}
	for nombre, tipo := range restricciones {
		var got string
		err := r.pool.QueryRow(context.Background(),
			`select contype::text from pg_constraint where conname = $1`, nombre).Scan(&got)
		if err != nil {
			t.Errorf("%s: %v", nombre, err)
			continue
		}
		if got != tipo {
			t.Errorf("%s es de tipo %q, se esperaba %q", nombre, got, tipo)
		}
	}
}

// Criterio 2: recien creada no esta publicada, y el publico ve 404.
func TestIntegracionUnaPaginaNuevaNoEstaPublicada(t *testing.T) {
	r := &Repo{pool: pool(t)}
	p := paginaDePrueba(t, r, "Nueva")

	if p.PublishedVersionId != nil || p.PublishedNumber != nil {
		t.Errorf("publicada = %v / %v, se esperaba nula", p.PublishedVersionId, p.PublishedNumber)
	}
	if p.Version != 1 || p.DraftNumber != 1 {
		t.Errorf("version = %d, borrador = %d", p.Version, p.DraftNumber)
	}
	if _, err := r.publica(context.Background(), p.Slug); !errors.Is(err, errNoExiste) {
		t.Errorf("publica = %v, se esperaba errNoExiste", err)
	}
}

// Criterio 3: guardar crea una fila nueva y no toca la publicada.
func TestIntegracionGuardarCreaUnaVersionYNoTocaLaPublicada(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	p := paginaDePrueba(t, r, "Primera")

	if _, err := r.publicar(ctx, p.Id, p.DraftVersionId); err != nil {
		t.Fatalf("publicar: %v", err)
	}

	g, err := r.guardar(ctx, p.Id, 1, guardado(p, "Segunda"))
	if err != nil {
		t.Fatalf("guardar: %v", err)
	}

	if g.Version != 2 || g.DraftNumber != 2 || g.Title != "Segunda" {
		t.Errorf("guardada: version %d, borrador %d, titulo %q", g.Version, g.DraftNumber, g.Title)
	}
	if g.DraftVersionId == p.DraftVersionId {
		t.Error("el guardado reutilizo la fila de la version anterior")
	}
	if g.PublishedVersionId == nil || *g.PublishedVersionId != p.DraftVersionId {
		t.Errorf("la publicada cambio al guardar: %v", g.PublishedVersionId)
	}
	if n := contarVersiones(t, r, p.Id); n != 2 {
		t.Errorf("versiones = %d, se esperaban 2", n)
	}

	pub, err := r.publica(context.Background(), p.Slug)
	if err != nil || pub.Title != "Primera" {
		t.Errorf("el publico ve %q (%v), se esperaba la version publicada", pub.Title, err)
	}
}

// Criterio 6: publicar una version anterior revierte sin borrar ninguna.
func TestIntegracionPublicarUnaAnteriorRevierteSinBorrar(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	p := paginaDePrueba(t, r, "Uno")

	g, err := r.guardar(ctx, p.Id, 1, guardado(p, "Dos"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.publicar(ctx, p.Id, g.DraftVersionId); err != nil {
		t.Fatal(err)
	}
	if pub, _ := r.publica(context.Background(), p.Slug); pub.Title != "Dos" {
		t.Fatalf("antes de revertir el publico ve %q", pub.Title)
	}

	revertida, err := r.publicar(ctx, p.Id, p.DraftVersionId)
	if err != nil {
		t.Fatalf("revertir: %v", err)
	}

	if pub, _ := r.publica(context.Background(), p.Slug); pub.Title != "Uno" {
		t.Errorf("tras revertir el publico ve %q, se esperaba Uno", pub.Title)
	}
	if n := contarVersiones(t, r, p.Id); n != 2 {
		t.Errorf("versiones = %d: revertir no puede borrar ninguna", n)
	}
	// Publicar no toca el borrador: ni su version ni su contenido.
	if revertida.Version != 2 || revertida.Title != "Dos" {
		t.Errorf("tras publicar: version %d, borrador %q", revertida.Version, revertida.Title)
	}
}

// Criterio 7: borrar la pagina se lleva sus versiones.
func TestIntegracionBorrarLaPaginaSeLlevaSusVersiones(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	p := paginaDePrueba(t, r, "Efimera")

	if _, err := r.guardar(ctx, p.Id, 1, guardado(p, "Efimera 2")); err != nil {
		t.Fatal(err)
	}
	// Publicada, para que el borrado tenga que pasar por la llave circular.
	if _, err := r.publicar(ctx, p.Id, p.DraftVersionId); err != nil {
		t.Fatal(err)
	}

	if err := r.borrar(ctx, p.Id); err != nil {
		t.Fatalf("borrar: %v", err)
	}
	if n := contarVersiones(t, r, p.Id); n != 0 {
		t.Errorf("quedaron %d versiones huerfanas", n)
	}
	if err := r.borrar(ctx, p.Id); !errors.Is(err, errNoExiste) {
		t.Errorf("borrar dos veces = %v, se esperaba errNoExiste", err)
	}
}

func TestIntegracionNoSePublicaLaVersionDeOtraPagina(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	a := paginaDePrueba(t, r, "A")
	b := paginaDePrueba(t, r, "B")

	if _, err := r.publicar(ctx, a.Id, b.DraftVersionId); !errors.Is(err, errVersionAjena) {
		t.Errorf("publicar ajena = %v, se esperaba errVersionAjena", err)
	}
	if _, err := r.publicar(ctx, uuid.New(), a.DraftVersionId); !errors.Is(err, errNoExiste) {
		t.Errorf("pagina inexistente = %v, se esperaba errNoExiste", err)
	}

	// Y la base lo impide aunque alguien se salte el repositorio.
	_, err := r.pool.Exec(context.Background(),
		`update pages set published_version_id = $1 where id = $2`, b.DraftVersionId, a.Id)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "pages_version_publicada_fk" {
		t.Errorf("un UPDATE a mano con la version de otra pagina = %v; la llave compuesta tenia que rechazarlo", err)
	}
}

func TestIntegracionUnIfMatchViejoNoEscribeYDistingueNoExiste(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	p := paginaDePrueba(t, r, "Uno")

	if _, err := r.guardar(ctx, p.Id, 1, guardado(p, "Dos")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.guardar(ctx, p.Id, 1, guardado(p, "Pisaria")); !errors.Is(err, errVersion) {
		t.Errorf("version vieja = %v, se esperaba errVersion", err)
	}
	if n := contarVersiones(t, r, p.Id); n != 2 {
		t.Errorf("versiones = %d: un 409 no puede dejar version escrita", n)
	}
	if _, err := r.guardar(ctx, uuid.New(), 1, guardado(p, "x")); !errors.Is(err, errNoExiste) {
		t.Errorf("pagina inexistente = %v, se esperaba errNoExiste", err)
	}
}

// Dos guardados con el mismo If-Match a la vez: gana uno, el otro es 409, y no
// quedan dos versiones con el mismo numero ni una version sin su guardado.
func TestIntegracionDosGuardadosALaVezSoloGanaUno(t *testing.T) {
	r := &Repo{pool: pool(t)}
	p := paginaDePrueba(t, r, "Base")

	const n = 5
	var (
		wg      sync.WaitGroup
		errores = make([]error, n)
		salida  = make(chan struct{})
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-salida
			_, errores[i] = r.guardar(conActor(idDeAna), p.Id, 1, guardado(p, fmt.Sprintf("Intento %d", i)))
		}()
	}
	close(salida)
	wg.Wait()

	ganadores := 0
	for i, err := range errores {
		switch {
		case err == nil:
			ganadores++
		case errors.Is(err, errVersion):
		default:
			t.Errorf("intento %d: %v", i, err)
		}
	}
	if ganadores != 1 {
		t.Errorf("ganadores = %d, se esperaba exactamente 1", ganadores)
	}
	if got := contarVersiones(t, r, p.Id); got != 2 {
		t.Errorf("versiones = %d, se esperaban 2 (la inicial y la ganadora)", got)
	}
}

func TestIntegracionUnSlugOcupadoEsErrSlugOcupado(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	a := paginaDePrueba(t, r, "A")
	b := paginaDePrueba(t, r, "B")

	if _, err := r.crear(ctx, guardado(a, "Otra con el mismo slug")); !errors.Is(err, errSlugOcupado) {
		t.Errorf("crear = %v, se esperaba errSlugOcupado", err)
	}
	cambio := guardado(b, "B")
	cambio.Slug = a.Slug
	if _, err := r.guardar(ctx, b.Id, 1, cambio); !errors.Is(err, errSlugOcupado) {
		t.Errorf("guardar = %v, se esperaba errSlugOcupado", err)
	}
	if n := contarVersiones(t, r, b.Id); n != 1 {
		t.Errorf("versiones de B = %d: el guardado que fallo por el slug no puede dejar version", n)
	}
}

// La auditoria se llena sola: la pagina con quien la creo y quien la toco
// despues, y cada version con quien la guardo.
func TestIntegracionLaAuditoriaSeLlenaSola(t *testing.T) {
	r := &Repo{pool: pool(t)}
	p := paginaDePrueba(t, r, "Uno")

	if _, err := r.guardar(conActor(idDeLuis), p.Id, 1, guardado(p, "Dos")); err != nil {
		t.Fatal(err)
	}

	var creo, toco pgtype.UUID
	if err := r.pool.QueryRow(context.Background(),
		`select created_by, updated_by from pages where id = $1`, p.Id).Scan(&creo, &toco); err != nil {
		t.Fatal(err)
	}
	if uuid.UUID(creo.Bytes).String() != idDeAna || uuid.UUID(toco.Bytes).String() != idDeLuis {
		t.Errorf("pagina: creo %v, toco %v", uuid.UUID(creo.Bytes), uuid.UUID(toco.Bytes))
	}

	versiones, _, err := r.versiones(context.Background(), p.Id, mustParams(t, url.Values{"sort": {"number,asc"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(versiones) != 2 || versiones[0].CreatedBy == nil || versiones[1].CreatedBy == nil ||
		versiones[0].CreatedBy.String() != idDeAna || versiones[1].CreatedBy.String() != idDeLuis {
		t.Errorf("versiones = %+v", versiones)
	}
}

// Sin actor, firma el sistema: no NULL.
func TestIntegracionSinActorFirmaElSistema(t *testing.T) {
	r := &Repo{pool: pool(t)}
	p, err := r.crear(context.Background(), borrador{
		Slug: fmt.Sprintf("prueba-sistema-%d", time.Now().UnixNano()), Title: "S", Blocks: []Bloque{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(context.Background(), `delete from pages where id = $1`, p.Id) })

	if p.UpdatedBy == nil || p.UpdatedBy.String() != audit.SystemActor {
		t.Errorf("updated_by = %v, se esperaba el actor de sistema", p.UpdatedBy)
	}
}

func TestIntegracionElListadoOrdenaYFiltraEnLaConsulta(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeAna)
	a := paginaDePrueba(t, r, "A")
	b := paginaDePrueba(t, r, "B")
	if _, err := r.publicar(ctx, b.Id, b.DraftVersionId); err != nil {
		t.Fatal(err)
	}

	sinPublicar, _, err := r.pagina(context.Background(), mustParamsPaginas(t, url.Values{"published": {"false"}, "size": {"100"}}))
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	vistas := map[uuid.UUID]PaginaResumen{}
	for _, s := range sinPublicar {
		vistas[s.Id] = s
		if s.PublishedNumber != nil {
			t.Errorf("%s salio en el filtro de no publicadas", s.Slug)
		}
	}
	if _, ok := vistas[a.Id]; !ok {
		t.Error("la pagina sin publicar no salio en su filtro")
	}
	if _, ok := vistas[b.Id]; ok {
		t.Error("la pagina publicada salio en el filtro de no publicadas")
	}

	porSlug, _, err := r.pagina(context.Background(), mustParamsPaginas(t, url.Values{"sort": {"slug,desc"}, "size": {"100"}}))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(porSlug); i++ {
		if porSlug[i-1].Slug < porSlug[i].Slug {
			t.Fatalf("no esta en orden descendente por slug: %q antes de %q", porSlug[i-1].Slug, porSlug[i].Slug)
		}
	}
}

// La semilla: la portada existe y esta publicada en un fork recien migrado.
// Si alguien la edito en su base local, la prueba sigue valiendo: solo mira
// que haya una version publicada.
func TestIntegracionLaPortadaSembradaEstaPublicada(t *testing.T) {
	r := &Repo{pool: pool(t)}
	pub, err := r.publica(context.Background(), "inicio")
	if errors.Is(err, errNoExiste) {
		t.Skip("la portada sembrada ya no esta publicada en esta base: se despublico o se borro a mano")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(pub.Blocks) == 0 {
		t.Error("la portada publicada no tiene bloques")
	}
	crudo, _ := json.Marshal(pub.Blocks)
	if string(crudo) == "null" {
		t.Error("los bloques salen como null")
	}
}

func mustParams(t *testing.T, q url.Values) paging.Params {
	t.Helper()
	p, prob := listadoDeVersiones.Parse(q)
	if prob != nil {
		t.Fatalf("parametros: %v", prob)
	}
	return p
}

func mustParamsPaginas(t *testing.T, q url.Values) paging.Params {
	t.Helper()
	p, prob := listadoDePaginas.Parse(q)
	if prob != nil {
		t.Fatalf("parametros: %v", prob)
	}
	return p
}

// Una pagina siempre tiene al menos una version, asi que "cero versiones" solo
// puede significar que la pagina no existe. Eso es 404, no un 200 vacio.
func TestIntegracionLasVersionesDeUnaPaginaQueNoExisteSonErrNoExiste(t *testing.T) {
	r := &Repo{pool: pool(t)}
	if _, _, err := r.versiones(context.Background(), uuid.New(), mustParams(t, url.Values{})); !errors.Is(err, errNoExiste) {
		t.Errorf("versiones = %v, se esperaba errNoExiste", err)
	}
}
