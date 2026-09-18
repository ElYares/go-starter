package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Estas pruebas son lo unico que ejecuta el SQL de verdad, y la deduplicacion
// concurrente SOLO existe en la base: la decide el indice unico, y un repo
// falso diria que si sin haber comprobado nada.
//
// Se saltan solas sin DATABASE_URL. En local van dentro del contenedor:
//
//	docker exec -w /workspace devherd-go-starter-<hash>-api-1 \
//	    go test ./internal/modules/media/ -run Integracion -v
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

const idDeQuienSube = "3f2a1b0c-9d8e-7000-8000-0a1b2c3d4e60"

func conActor() context.Context {
	return rbac.WithActor(context.Background(), rbac.Actor{ID: idDeQuienSube})
}

// nuevoAlAzar es una fila con un hash que no puede existir ya, y que se borra
// al terminar: la base es la de desarrollo, y no se deja basura en ella.
func nuevoAlAzar(t *testing.T, r *Repo) nuevo {
	t.Helper()
	semilla := make([]byte, 32)
	if _, err := rand.Read(semilla); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(semilla)
	nombre := "prueba.png"

	t.Cleanup(func() {
		if _, err := r.pool.Exec(context.Background(), `delete from media where sha256 = $1`, sum[:]); err != nil {
			t.Errorf("limpiando: %v", err)
		}
	})
	return nuevo{sha256: sum[:], mime: "image/png", tamano: 10, ancho: 2, alto: 3, nombre: &nombre, llave: llaveDe(sum[:])}
}

func contarPorHash(t *testing.T, r *Repo, sum []byte) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), `select count(*) from media where sha256 = $1`, sum).Scan(&n); err != nil {
		t.Fatalf("contando: %v", err)
	}
	return n
}

func TestIntegracionInsertarGuardaLaFilaConSuAuditoria(t *testing.T) {
	r := &Repo{pool: pool(t)}
	n := nuevoAlAzar(t, r)

	reg, creado, err := r.insertar(conActor(), n)
	if err != nil {
		t.Fatalf("insertar: %v", err)
	}
	if !creado {
		t.Fatal("la primera insercion dice que ya existia")
	}
	if reg.CreatedBy == nil || reg.CreatedBy.String() != idDeQuienSube {
		t.Errorf("createdBy = %v: la auditoria no se lleno sola", reg.CreatedBy)
	}
	if reg.Width != 2 || reg.Height != 3 || reg.SizeBytes != 10 || reg.llave != n.llave {
		t.Errorf("fila = %+v", reg)
	}

	leido, err := r.obtener(context.Background(), reg.Id)
	if err != nil || leido.Sha256 != reg.Sha256 || leido.Url != urlPublica(reg.Id) {
		t.Errorf("obtener = %+v, %v", leido, err)
	}
}

func TestIntegracionElMismoHashDevuelveLaFilaExistente(t *testing.T) {
	r := &Repo{pool: pool(t)}
	n := nuevoAlAzar(t, r)

	primera, _, err := r.insertar(conActor(), n)
	if err != nil {
		t.Fatalf("primera: %v", err)
	}
	otroNombre := "otro.png"
	n.nombre = &otroNombre

	segunda, creado, err := r.insertar(conActor(), n)
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}
	if creado || segunda.Id != primera.Id {
		t.Errorf("creado = %v, id = %s; se esperaba la fila de la primera (%s)", creado, segunda.Id, primera.Id)
	}
	if segunda.OriginalName == nil || *segunda.OriginalName != "prueba.png" {
		t.Errorf("originalName = %v: la segunda subida no cambia el nombre", segunda.OriginalName)
	}
	if c := contarPorHash(t, r, n.sha256); c != 1 {
		t.Errorf("hay %d filas con el mismo hash", c)
	}
}

// El criterio de la historia: dos subidas simultaneas del mismo archivo son una
// fila, una responde 201 y la otra 200, con el mismo id. Con un select previo
// en lugar del `on conflict`, las dos ven "no existe" y la segunda choca con el
// indice: un 500.
//
// Una carrera no sale en cada intento: la ventana entre mirar y escribir es de
// microsegundos. Por eso son varias rondas de muchas subidas a la vez. Con el
// select previo, esta prueba fallo en todas las corridas en que se probo.
func TestIntegracionSubidasSimultaneasDelMismoArchivoSonUnaSolaFila(t *testing.T) {
	r := &Repo{pool: pool(t)}

	const (
		rondas      = 20
		simultaneas = 16
	)
	for ronda := range rondas {
		n := nuevoAlAzar(t, r)

		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			ids      = map[string]bool{}
			nuevas   int
			errores  []error
			arrancar = make(chan struct{})
		)
		for range simultaneas {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-arrancar
				reg, creado, err := r.insertar(conActor(), n)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errores = append(errores, err)
					return
				}
				ids[reg.Id.String()] = true
				if creado {
					nuevas++
				}
			}()
		}
		close(arrancar)
		wg.Wait()

		if len(errores) > 0 {
			t.Fatalf("ronda %d: %d subidas fallaron; la primera: %v", ronda, len(errores), errores[0])
		}
		if nuevas != 1 || len(ids) != 1 {
			t.Fatalf("ronda %d: %d subidas dicen haber creado la fila y hay %d ids; se esperaba 1 y 1", ronda, nuevas, len(ids))
		}
		if c := contarPorHash(t, r, n.sha256); c != 1 {
			t.Fatalf("ronda %d: hay %d filas con el mismo hash", ronda, c)
		}
	}
}

// La base se niega a lo que el service ya rechaza. Es el cinturon de una fila
// escrita a mano o por otro camino.
func TestIntegracionLaBaseRechazaUnTipoQueNoSeAcepta(t *testing.T) {
	r := &Repo{pool: pool(t)}
	n := nuevoAlAzar(t, r)
	n.mime = "image/svg+xml"

	if _, _, err := r.insertar(conActor(), n); err == nil {
		t.Error("la base acepto un SVG")
	}
}
