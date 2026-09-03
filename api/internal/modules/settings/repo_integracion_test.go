package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/audit"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Estas pruebas son lo unico que ejecuta el SQL de verdad.
//
// Las de settings_test.go afirman sobre lo que el service le PIDE al
// repositorio; ninguna comprueba que la consulta que se arma sea valida, ni que
// Postgres haga lo que se espera con ella. Un `order by` mal concatenado o un
// marcador mal numerado pasa todas aquellas y falla en la primera peticion
// real.
//
// Se saltan solas sin DATABASE_URL, que es el caso de CI: ahi no hay base. Para
// correrlas:
//
//	devherd exec go-starter api -- go test ./internal/modules/settings/ -run Integracion -v
func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("sin DATABASE_URL: la prueba de integracion necesita una base real")
	}

	ctx := context.Background()
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("conectando: %v", err)
	}
	if err := p.Ping(ctx); err != nil {
		t.Fatalf("la base no responde: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

const otroID = "8c2d4e6f-1a3b-4c5d-9e7f-0a1b2c3d4e5f"

// conActor devuelve un contexto con sesion, que es lo que lee platform/audit.
func conActor(id string) context.Context {
	return rbac.WithActor(context.Background(), rbac.Actor{ID: id})
}

// claveDePrueba se limpia sola. Sin esto, la segunda ejecucion chocaria con la
// primera y el fallo diria "ya existe" en vez de lo que se estaba probando.
func claveDePrueba(t *testing.T, r *Repo) string {
	t.Helper()
	key := fmt.Sprintf("prueba.integracion.k%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = r.pool.Exec(context.Background(), `delete from settings where key = $1`, key)
	})
	return key
}

// auditoriaDe lee las cuatro columnas directamente. Setting no expone las de
// creacion a proposito, y son justo las que hay que vigilar.
func auditoriaDe(t *testing.T, r *Repo, key string) (createdAt, updatedAt time.Time, createdBy, updatedBy pgtype.UUID) {
	t.Helper()
	err := r.pool.QueryRow(context.Background(),
		`select created_at, updated_at, created_by, updated_by from settings where key = $1`, key).
		Scan(&createdAt, &updatedAt, &createdBy, &updatedBy)
	if err != nil {
		t.Fatalf("leyendo la auditoria de %q: %v", key, err)
	}
	return
}

func uuidDe(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("uuid invalido: %v", err)
	}
	return u
}

func TestIntegracionElAltaLlenaLaAuditoriaSola(t *testing.T) {
	r := &Repo{pool: pool(t)}
	key := claveDePrueba(t, r)
	ctx := conActor(idDeJuan)

	creada, err := r.crear(ctx, Setting{Key: key, Value: json.RawMessage(`{"a":1}`), IsPublic: true})
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	if creada.Version != 1 {
		t.Errorf("version = %d, se esperaba 1", creada.Version)
	}

	createdAt, updatedAt, createdBy, updatedBy := auditoriaDe(t, r, key)
	quien := uuidDe(t, idDeJuan)

	if createdBy != quien || updatedBy != quien {
		t.Errorf("created_by/updated_by = %v/%v; el service nunca los asigno y aun asi tienen que estar", createdBy, updatedBy)
	}
	if !createdAt.Equal(updatedAt) {
		t.Errorf("created_at = %v, updated_at = %v; una fila recien creada no puede parecer modificada", createdAt, updatedAt)
	}
}

// El corazon de la auditoria: modificar no puede borrar quien creo la fila.
func TestIntegracionLaModificacionConservaLaCreacion(t *testing.T) {
	r := &Repo{pool: pool(t)}
	key := claveDePrueba(t, r)

	if _, err := r.crear(conActor(idDeJuan), Setting{Key: key, Value: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatalf("crear: %v", err)
	}
	createdAtAntes, _, createdByAntes, _ := auditoriaDe(t, r, key)

	// Otra persona guarda encima.
	actualizada, err := r.actualizar(conActor(otroID), Setting{
		Key: key, Value: json.RawMessage(`{"a":2}`), IsPublic: true, Version: 1,
	})
	if err != nil {
		t.Fatalf("actualizar: %v", err)
	}
	if actualizada.Version != 2 {
		t.Errorf("version = %d; el guardado tiene que subirla", actualizada.Version)
	}

	createdAtDespues, updatedAt, createdByDespues, updatedBy := auditoriaDe(t, r, key)

	if createdByDespues != createdByAntes || !createdAtDespues.Equal(createdAtAntes) {
		t.Errorf("la modificacion piso la creacion: created_by %v -> %v", createdByAntes, createdByDespues)
	}
	if updatedBy != uuidDe(t, otroID) {
		t.Errorf("updated_by = %v, se esperaba el segundo actor", updatedBy)
	}
	if !updatedAt.After(createdAtDespues) {
		t.Errorf("updated_at (%v) no avanzo respecto de created_at (%v)", updatedAt, createdAtDespues)
	}
}

// La comprobacion de version y la escritura van en la misma sentencia. Esta es
// la prueba de que Postgres rechaza la version vieja, no el codigo de Go.
func TestIntegracionUnaVersionViejaNoEscribe(t *testing.T) {
	r := &Repo{pool: pool(t)}
	key := claveDePrueba(t, r)
	ctx := conActor(idDeJuan)

	if _, err := r.crear(ctx, Setting{Key: key, Value: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatalf("crear: %v", err)
	}
	if _, err := r.actualizar(ctx, Setting{Key: key, Value: json.RawMessage(`{"a":2}`), Version: 1}); err != nil {
		t.Fatalf("primer guardado: %v", err)
	}

	// Alguien que leyo la version 1 intenta guardar cuando ya va por la 2.
	_, err := r.actualizar(ctx, Setting{Key: key, Value: json.RawMessage(`{"a":99}`), Version: 1})
	if !errors.Is(err, errVersion) {
		t.Fatalf("err = %v, se esperaba errVersion", err)
	}

	actual, err := r.obtener(ctx, key)
	if err != nil {
		t.Fatalf("obtener: %v", err)
	}
	if string(actual.Value) != `{"a": 2}` && string(actual.Value) != `{"a":2}` {
		t.Errorf("value = %s; el guardado rechazado no puede haber escrito nada", actual.Value)
	}
}

func TestIntegracionDistingueNoExisteDeVersionEquivocada(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeJuan)

	_, err := r.actualizar(ctx, Setting{
		Key: "prueba.integracion.no.existe", Value: json.RawMessage(`1`), Version: 1,
	})
	if !errors.Is(err, errNoExiste) {
		t.Fatalf("err = %v, se esperaba errNoExiste; 404 y 409 llevan al usuario a cosas distintas", err)
	}
}

func TestIntegracionUnaClaveDuplicadaEsErrYaExiste(t *testing.T) {
	r := &Repo{pool: pool(t)}
	key := claveDePrueba(t, r)
	ctx := conActor(idDeJuan)

	if _, err := r.crear(ctx, Setting{Key: key, Value: json.RawMessage(`1`)}); err != nil {
		t.Fatalf("crear: %v", err)
	}
	if _, err := r.crear(ctx, Setting{Key: key, Value: json.RawMessage(`2`)}); !errors.Is(err, errYaExiste) {
		t.Fatalf("err = %v, se esperaba errYaExiste", err)
	}
}

// Sin actor, firma el sistema. Es el caso de un seed o una tarea de fondo, y la
// columna no puede quedar en NULL.
func TestIntegracionSinActorFirmaElSistema(t *testing.T) {
	r := &Repo{pool: pool(t)}
	key := claveDePrueba(t, r)

	if _, err := r.crear(context.Background(), Setting{Key: key, Value: json.RawMessage(`1`)}); err != nil {
		t.Fatalf("crear: %v", err)
	}

	_, _, createdBy, _ := auditoriaDe(t, r, key)
	if !createdBy.Valid {
		t.Fatal("created_by quedo en NULL: 'no se sabe' y 'lo hizo el sistema' no son lo mismo")
	}
	if createdBy != uuidDe(t, audit.SystemActor) {
		t.Errorf("created_by = %v, se esperaba el actor de sistema", createdBy)
	}
}

// Lo que ninguna prueba sin base puede comprobar: que el SQL que arma paging
// sea SQL valido y ordene de verdad.
func TestIntegracionElOrdenYElFiltroSeAplicanEnLaConsulta(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeJuan)

	for _, k := range []string{"a", "b", "c"} {
		key := claveDePrueba(t, r)
		if _, err := r.crear(ctx, Setting{Key: key, Value: json.RawMessage(`"` + k + `"`), IsPublic: true}); err != nil {
			t.Fatalf("crear: %v", err)
		}
	}

	casos := []struct{ nombre, query string }{
		{"sin parametros", ""},
		{"orden descendente", "sort=key,desc"},
		{"orden por fecha", "sort=updatedAt,desc"},
		{"con filtro", "isPublic=true"},
		{"filtro y orden", "isPublic=true&sort=updatedAt,desc&size=5&page=1"},
		{"tamano topado", "size=1000000"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			q, err := parsearQuery(c.query)
			if err != nil {
				t.Fatal(err)
			}
			p, prob := listado.Parse(q)
			if prob != nil {
				t.Fatalf("no se esperaba problema: %+v", prob)
			}

			items, total, err := r.pagina(ctx, p)
			if err != nil {
				t.Fatalf("la consulta no es SQL valido: %v", err)
			}
			if len(items) > p.Limit() {
				t.Errorf("llegaron %d filas con limit %d", len(items), p.Limit())
			}
			if total < 3 {
				t.Errorf("total = %d; deberian estar al menos las tres recien creadas", total)
			}
		})
	}
}

// El orden descendente por clave tiene que salir descendente de la base, no de
// una ordenacion en Go despues de traer la pagina.
func TestIntegracionElOrdenLoHaceLaBase(t *testing.T) {
	r := &Repo{pool: pool(t)}
	ctx := conActor(idDeJuan)

	q, _ := parsearQuery("sort=key,desc&size=100")
	p, _ := listado.Parse(q)

	items, _, err := r.pagina(ctx, p)
	if err != nil {
		t.Fatalf("pagina: %v", err)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].Key < items[i].Key {
			t.Fatalf("la pagina no vino ordenada: %q antes que %q", items[i-1].Key, items[i].Key)
		}
	}
}

func parsearQuery(s string) (url.Values, error) { return url.ParseQuery(s) }
