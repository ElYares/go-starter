package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/elyares/go-starter/api/internal/platform/rbac"
)

// Estas pruebas son lo unico que ejecuta el SQL de verdad, y aqui hay dos cosas
// que SOLO existen en la base:
//
//   - `citext`, que es quien decide que Ana@casa.com y ana@casa.com son la
//     misma cuenta. En Go no hay nada que probar de eso
//   - la invariante del ultimo admin, que vive en el WHERE de la sentencia que
//     escribe. Un doble diria que si sin haber comprobado nada
//
// Se saltan solas sin DATABASE_URL. Para correrlas hace falta la base del
// stack local, asi que van dentro del contenedor:
//
//	docker exec -w /workspace devherd-go-starter-<hash>-api-1 \
//	    go test ./internal/modules/identity/ -run Integracion -v
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

const idDeQuienSiembra = "3f2a1b0c-9d8e-7000-8000-0a1b2c3d4e5f"

func conActor(id string) context.Context {
	return rbac.WithActor(context.Background(), rbac.Actor{ID: id})
}

func servicioReal(t *testing.T) (*Service, *Repo) {
	t.Helper()
	r := &Repo{pool: pool(t)}
	return &Service{repo: r, dev: true}, r
}

// correoDePrueba se limpia solo. Sin esto, la segunda ejecucion chocaria con la
// primera y el fallo diria "ya existe" en vez de lo que se estaba probando.
func correoDePrueba(t *testing.T, r *Repo) string {
	t.Helper()
	email := fmt.Sprintf("prueba.integracion.u%d@casa.com", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = r.pool.Exec(context.Background(), `delete from users where email = $1`, email)
	})
	return email
}

func crearDePrueba(t *testing.T, s *Service, r *Repo, roles ...string) Usuario {
	t.Helper()
	email := correoDePrueba(t, r)

	u, err := s.Crear(conActor(idDeQuienSiembra), UsuarioNuevo{
		Email: email, Password: claveDePrueba, DisplayName: nombreDePrueba, Roles: roles,
	})
	if err != nil {
		t.Fatalf("creando el usuario de prueba: %v", err)
	}
	return u
}

// El motivo de que `email` sea `citext` y no `text`. Con `text`, esta prueba
// pasa —son dos cadenas distintas— y el resultado es dos cuentas para la misma
// persona, cada una con su contrasena.
func TestIntegracionElCorreoNoDistingueMayusculas(t *testing.T) {
	s, r := servicioReal(t)

	primero := crearDePrueba(t, s, r)
	repetido := strings.ToUpper(primero.Email)

	_, err := s.Crear(conActor(idDeQuienSiembra), UsuarioNuevo{
		Email: repetido, Password: claveDePrueba, DisplayName: nombreDePrueba,
	})
	if err == nil {
		// La limpieza va por el correo original, asi que esta fila se quedaria.
		_, _ = r.pool.Exec(context.Background(), `delete from users where email = $1`, repetido)
		t.Fatalf("la base acepto %q teniendo ya %q: son la misma cuenta", repetido, primero.Email)
	}

	if p := problemaDe(t, err); p.Status != 409 {
		t.Errorf("estado = %d, se esperaba 409", p.Status)
	}
}

// Y la mitad complementaria: buscar por otra capitalizacion tiene que encontrar
// la fila. Un unique insensible con una busqueda sensible da el bug al reves:
// no puedes registrarte y tampoco puedes entrar.
func TestIntegracionSeEntraConOtraCapitalizacionDelCorreo(t *testing.T) {
	s, r := servicioReal(t)
	creado := crearDePrueba(t, s, r)

	u, err := s.Autenticar(context.Background(), strings.ToUpper(creado.Email), claveDePrueba)
	if err != nil {
		t.Fatalf("Autenticar con el correo en mayusculas: %v", err)
	}
	if u.ID != creado.ID {
		t.Errorf("id = %q, se esperaba %q", u.ID, creado.ID)
	}
}

// La contrasena no puede estar en NINGUNA columna, no solo en la que se mira.
// Se recorre la fila entera a proposito: el dia que alguien agregue una columna
// `password_recovery_hint` con el valor dentro, esta prueba lo caza.
func TestIntegracionLaContrasenaNoQuedaEnClaroEnNingunaColumna(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	rows, err := r.pool.Query(context.Background(),
		`select to_jsonb(users) from users where id = $1`, u.ID)
	if err != nil {
		t.Fatalf("leyendo la fila: %v", err)
	}
	defer rows.Close()

	fila, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("leyendo la fila: %v", err)
	}

	if strings.Contains(fila, claveDePrueba) {
		t.Fatalf("la contrasena esta en claro en la fila: %s", fila)
	}
	if !strings.Contains(fila, "$argon2id$") {
		t.Errorf("no hay un hash argon2id en la fila: %s", fila)
	}
}

func TestIntegracionElAltaLlenaLaAuditoriaSola(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	var (
		createdAt, updatedAt time.Time
		createdBy, updatedBy pgtype.UUID
	)
	err := r.pool.QueryRow(context.Background(),
		`select created_at, updated_at, created_by, updated_by from users where id = $1`, u.ID).
		Scan(&createdAt, &updatedAt, &createdBy, &updatedBy)
	if err != nil {
		t.Fatalf("leyendo la auditoria: %v", err)
	}

	var quien pgtype.UUID
	if err := quien.Scan(idDeQuienSiembra); err != nil {
		t.Fatal(err)
	}
	if createdBy != quien || updatedBy != quien {
		t.Errorf("created_by/updated_by = %v/%v; el service nunca los asigno y aun asi tienen que estar", createdBy, updatedBy)
	}
	if !createdAt.Equal(updatedAt) {
		t.Errorf("created_at = %v, updated_at = %v; una fila recien creada no puede parecer modificada", createdAt, updatedAt)
	}
}

func TestIntegracionLosCuatroRolesDelStarterEstanSembrados(t *testing.T) {
	r := &Repo{pool: pool(t)}

	rows, err := r.pool.Query(context.Background(), `select key from roles order by key`)
	if err != nil {
		t.Fatalf("leyendo los roles: %v", err)
	}
	defer rows.Close()

	claves, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("leyendo los roles: %v", err)
	}

	for _, esperado := range []string{RolSuperadmin, RolAdmin, RolStaff, RolViewer} {
		if !slices.Contains(claves, esperado) {
			t.Errorf("falta el rol %q; hay %v", esperado, claves)
		}
	}
}

// catalogoIntacto guarda el catalogo y lo repone al terminar. Las pruebas de
// siembra reconcilian la tabla entera: sin esto, la primera dejaria la base sin
// los permisos de los demas modulos y las siguientes correrian sobre un estado
// que no es el de nadie.
func catalogoIntacto(t *testing.T, r *Repo) {
	t.Helper()
	ctx := context.Background()

	rows, err := r.pool.Query(ctx, `select key, description, sensitive from permissions order by key`)
	if err != nil {
		t.Fatalf("leyendo el catalogo: %v", err)
	}
	previos, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rbac.Permission, error) {
		var p rbac.Permission
		err := row.Scan(&p.Key, &p.Desc, &p.Sensitive)
		return p, err
	})
	rows.Close()
	if err != nil {
		t.Fatalf("leyendo el catalogo: %v", err)
	}

	concesiones := filasDeRol(t, r, "role_permissions")
	ofertas := filasDeRol(t, r, "role_permission_offers")

	t.Cleanup(func() {
		if err := r.SembrarPermisos(ctx, previos); err != nil {
			t.Errorf("reponiendo el catalogo: %v", err)
		}
		// Resembrar no basta: el catalogo de prueba se llevo por cascada las
		// ofertas del catalogo real, y la siembra se lo volveria a ofrecer todo
		// al admin, incluido lo que alguien le quito a proposito en su base.
		reponerFilasDeRol(t, r, "role_permissions", concesiones)
		reponerFilasDeRol(t, r, "role_permission_offers", ofertas)
	})
}

// filaDeRol es una fila de role_permissions o de role_permission_offers: las
// dos tablas tienen la misma forma.
type filaDeRol struct {
	rol     string
	permiso string
}

func filasDeRol(t *testing.T, r *Repo, tabla string) []filaDeRol {
	t.Helper()
	rows, err := r.pool.Query(context.Background(),
		`select role_id::text, permission_key from `+tabla)
	if err != nil {
		t.Fatalf("leyendo %s: %v", tabla, err)
	}
	filas, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (filaDeRol, error) {
		var f filaDeRol
		err := row.Scan(&f.rol, &f.permiso)
		return f, err
	})
	if err != nil {
		t.Fatalf("leyendo %s: %v", tabla, err)
	}
	return filas
}

func reponerFilasDeRol(t *testing.T, r *Repo, tabla string, filas []filaDeRol) {
	t.Helper()
	ctx := context.Background()
	roles := make([]string, len(filas))
	permisos := make([]string, len(filas))
	for i, f := range filas {
		roles[i], permisos[i] = f.rol, f.permiso
	}
	if _, err := r.pool.Exec(ctx, `delete from `+tabla); err != nil {
		t.Errorf("vaciando %s: %v", tabla, err)
		return
	}
	if _, err := r.pool.Exec(ctx,
		`insert into `+tabla+` (role_id, permission_key)
		 select r::uuid, p from unnest($1::text[], $2::text[]) as t(r, p)`,
		roles, permisos); err != nil {
		t.Errorf("reponiendo %s: %v", tabla, err)
	}
}

// Uno de cada clase, que es lo que hace comprobable el reparto entre los dos
// roles: el sensible es solo del superadmin, el otro llega tambien al admin.
var permisosDePrueba = []rbac.Permission{
	{Key: "prueba.integracion.leer", Desc: "Ver"},
	{Key: "prueba.integracion.escribir", Desc: "Escribir"},
	{Key: "prueba.integracion.repartir", Desc: "Repartir poder", Sensitive: true},
}

var noSensiblesDePrueba = []string{"prueba.integracion.escribir", "prueba.integracion.leer"}

func clavesDelCatalogo(t *testing.T, r *Repo) []string {
	t.Helper()
	rows, err := r.pool.Query(context.Background(), `select key from permissions order by key`)
	if err != nil {
		t.Fatalf("leyendo el catalogo: %v", err)
	}
	defer rows.Close()

	claves, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("leyendo el catalogo: %v", err)
	}
	return claves
}

// "Volver a arrancar no los duplica" es un criterio de la historia, y la unica
// forma de comprobarlo es sembrar dos veces y contar.
func TestIntegracionSembrarLosPermisosDosVecesNoDuplicaNada(t *testing.T) {
	r := &Repo{pool: pool(t)}
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("primera siembra: %v", err)
	}
	primera := clavesDelCatalogo(t, r)

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("segunda siembra: %v", err)
	}
	segunda := clavesDelCatalogo(t, r)

	if strings.Join(primera, ",") != strings.Join(segunda, ",") {
		t.Errorf("el catalogo cambio entre siembras:\n  %v\n  %v", primera, segunda)
	}
	if len(segunda) != len(permisosDePrueba) {
		t.Errorf("hay %d permisos, se declararon %d: %v", len(segunda), len(permisosDePrueba), segunda)
	}
}

// La otra mitad de la reconciliacion. Es lo que impide que borrar un modulo
// deje filas nombrando permisos que ya no existen, y roles que los conceden.
func TestIntegracionSembrarBorraLosPermisosQueYaNadieDeclara(t *testing.T) {
	r := &Repo{pool: pool(t)}
	catalogoIntacto(t, r)
	ctx := context.Background()

	conElViejo := append(slices.Clone(permisosDePrueba),
		rbac.Permission{Key: "prueba.integracion.modulo.borrado", Desc: "De un modulo que ya no esta"})
	if err := r.SembrarPermisos(ctx, conElViejo); err != nil {
		t.Fatalf("sembrando: %v", err)
	}

	// Y alguien se lo habia concedido al admin, que es el caso que importa: el
	// borrado tiene que llevarse tambien la concesion.
	if !slices.Contains(clavesDelCatalogo(t, r), "prueba.integracion.modulo.borrado") {
		t.Fatal("la preparacion no dejo el permiso viejo en la tabla")
	}

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("resembrando sin el modulo: %v", err)
	}

	if slices.Contains(clavesDelCatalogo(t, r), "prueba.integracion.modulo.borrado") {
		t.Error("el permiso del modulo borrado sigue en el catalogo")
	}

	var concesiones int
	err := r.pool.QueryRow(ctx,
		`select count(*) from role_permissions where permission_key = $1`,
		"prueba.integracion.modulo.borrado").Scan(&concesiones)
	if err != nil {
		t.Fatalf("contando concesiones: %v", err)
	}
	if concesiones != 0 {
		t.Errorf("quedaron %d concesiones de un permiso que ya no existe", concesiones)
	}
}

// Sin esto, el permiso de un modulo nuevo nace inalcanzable: nadie lo tiene, y
// la pantalla para concederlo tambien lo exige.
func TestIntegracionElSuperadminRecibeTodoPermisoDeclarado(t *testing.T) {
	s, r := servicioReal(t)
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("sembrando: %v", err)
	}

	super := crearDePrueba(t, s, r, RolSuperadmin)
	actor, err := s.Actor(ctx, super.ID)
	if err != nil {
		t.Fatalf("Actor: %v", err)
	}

	for _, p := range permisosDePrueba {
		if !actor.Can(p.Key) {
			t.Errorf("el superadmin no tiene %q; tiene %v", p.Key, actor.Permissions)
		}
	}
	if len(actor.Permissions) != len(permisosDePrueba) {
		t.Errorf("el superadmin tiene %v; el catalogo declara %d", actor.Permissions, len(permisosDePrueba))
	}
}

// La separacion que pidio el cambio: el admin opera el sitio, el superadmin
// reparte poder. Si el admin recibiera `identity.role.assign` podria darse a si
// mismo cualquier permiso, y los dos roles serian el mismo con otro nombre.
func TestIntegracionElAdminRecibeSoloLosPermisosNoSensibles(t *testing.T) {
	s, r := servicioReal(t)
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("sembrando: %v", err)
	}

	admin := crearDePrueba(t, s, r, RolAdmin)
	actor, err := s.Actor(ctx, admin.ID)
	if err != nil {
		t.Fatalf("Actor: %v", err)
	}

	for _, clave := range noSensiblesDePrueba {
		if !actor.Can(clave) {
			t.Errorf("el admin no tiene %q; tiene %v", clave, actor.Permissions)
		}
	}
	if actor.Can("prueba.integracion.repartir") {
		t.Error("el admin recibio un permiso marcado como sensible")
	}
}

// La condicion que impide que la siembra pelee con el dashboard: lo que se le
// quita al admin desde la pantalla de roles tiene que quedar quitado, y no
// volver en el siguiente despliegue.
func TestIntegracionLaSiembraNoReponeLoQueSeLeQuitoAlAdmin(t *testing.T) {
	r := &Repo{pool: pool(t)}
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("primera siembra: %v", err)
	}

	if _, err := r.pool.Exec(ctx,
		`delete from role_permissions rp using roles ro
		  where ro.id = rp.role_id and ro.key = $1 and rp.permission_key = $2`,
		RolAdmin, "prueba.integracion.escribir"); err != nil {
		t.Fatalf("revocando: %v", err)
	}

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("segunda siembra: %v", err)
	}

	var repuesto bool
	err := r.pool.QueryRow(ctx,
		`select exists (
		     select 1 from role_permissions rp join roles ro on ro.id = rp.role_id
		      where ro.key = $1 and rp.permission_key = $2)`,
		RolAdmin, "prueba.integracion.escribir").Scan(&repuesto)
	if err != nil {
		t.Fatalf("comprobando: %v", err)
	}
	if repuesto {
		t.Error("la siembra repuso un permiso que se le habia quitado al admin")
	}
}

// El problema que resolvio la oferta: un modulo que llega a una instalacion que
// ya arranco. Con la regla anterior —sembrar al admin solo si no tenia nada—
// sus permisos se quedaban en el superadmin, y la pantalla para concederlos no
// existia todavia.
func TestIntegracionElAdminRecibeLosPermisosDeUnModuloNuevo(t *testing.T) {
	s, r := servicioReal(t)
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("primera siembra: %v", err)
	}

	conElNuevo := append(slices.Clone(permisosDePrueba),
		rbac.Permission{Key: "prueba.integracion.nuevo.usar", Desc: "De un modulo que llega despues"},
		rbac.Permission{Key: "prueba.integracion.nuevo.repartir", Desc: "Delicado", Sensitive: true})
	if err := r.SembrarPermisos(ctx, conElNuevo); err != nil {
		t.Fatalf("siembra con el modulo nuevo: %v", err)
	}

	admin := crearDePrueba(t, s, r, RolAdmin)
	actor, err := s.Actor(ctx, admin.ID)
	if err != nil {
		t.Fatalf("Actor: %v", err)
	}
	if !actor.Can("prueba.integracion.nuevo.usar") {
		t.Errorf("el admin no recibio el permiso del modulo nuevo; tiene %v", actor.Permissions)
	}
	if actor.Can("prueba.integracion.nuevo.repartir") {
		t.Error("el admin recibio un permiso sensible del modulo nuevo")
	}
}

// Un viewer no hereda nada por existir. Es el caso negativo de los dos
// anteriores: sin el, un `cross join` sin `where` los pasaria todos.
func TestIntegracionUnViewerNoRecibeNingunPermiso(t *testing.T) {
	s, r := servicioReal(t)
	catalogoIntacto(t, r)
	ctx := context.Background()

	if err := r.SembrarPermisos(ctx, permisosDePrueba); err != nil {
		t.Fatalf("sembrando: %v", err)
	}

	viewer := crearDePrueba(t, s, r, RolViewer)
	actor, err := s.Actor(ctx, viewer.ID)
	if err != nil {
		t.Fatalf("Actor: %v", err)
	}
	if len(actor.Permissions) != 0 {
		t.Errorf("el viewer tiene %v; no se le concedio ninguno", actor.Permissions)
	}
}

// La invariante, contra la base. Aqui es donde significa algo: el WHERE de la
// sentencia es quien la aplica, y es lo unico que la hace atomica.
func TestIntegracionNoSeLePuedeQuitarElRolAlUltimoSuperadmin(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	super := crearDePrueba(t, s, r, RolSuperadmin)
	soloEsteSuperadmin(t, r, super.ID)

	err := r.quitarRol(ctx, super.ID, RolSuperadmin)
	if !errors.Is(err, errUltimoSuperadmin) {
		t.Fatalf("error = %v; se esperaba el del ultimo superadmin", err)
	}

	roles, err := r.rolesDe(ctx, super.ID)
	if err != nil {
		t.Fatalf("rolesDe: %v", err)
	}
	if !slices.Contains(roles, RolSuperadmin) {
		t.Error("el rol se quito igual: la sentencia dijo que no y borro")
	}
}

// El admin NO lleva invariante: es un rol como cualquier otro. Quitarselo al
// unico que lo tiene es una operacion normal, y bloquearla seria un 409 que
// nadie entiende.
func TestIntegracionAlUltimoAdminSiSeLePuedeQuitarElRol(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	admin := crearDePrueba(t, s, r, RolAdmin)
	if err := r.quitarRol(ctx, admin.ID, RolAdmin); err != nil {
		t.Fatalf("quitarle el rol admin al unico admin: %v", err)
	}
}

func TestIntegracionNoSePuedeDeshabilitarAlUltimoSuperadmin(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	super := crearDePrueba(t, s, r, RolSuperadmin)
	soloEsteSuperadmin(t, r, super.ID)

	_, err := r.deshabilitar(ctx, super.ID)
	if !errors.Is(err, errUltimoSuperadmin) {
		t.Fatalf("error = %v; se esperaba el del ultimo superadmin", err)
	}

	u, err := r.porID(ctx, super.ID)
	if err != nil {
		t.Fatalf("porID: %v", err)
	}
	if !u.Enabled {
		t.Error("quedo deshabilitado igual: la sentencia dijo que no y escribio")
	}
}

// El caso positivo, que es el que hace que las dos pruebas de arriba prueben
// algo: con otro admin habilitado, las dos operaciones pasan. Sin esta, un
// `where false` las dejaria a las dos en verde.
//
// El rol se le devuelve al primero antes de deshabilitar al segundo, y no es
// un detalle: sin eso, quitarle el rol al primero deja al segundo siendo el
// ultimo admin, y la invariante lo rechaza —con razon—. La primera version de
// esta prueba fallaba justo ahi.
func TestIntegracionConOtroSuperadminHabilitadoLasDosOperacionesPasan(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	primero := crearDePrueba(t, s, r, RolSuperadmin)
	segundo := crearDePrueba(t, s, r, RolSuperadmin)
	soloEsteSuperadmin(t, r, primero.ID, segundo.ID)

	if err := r.quitarRol(ctx, primero.ID, RolSuperadmin); err != nil {
		t.Fatalf("quitarRol teniendo otro superadmin habilitado: %v", err)
	}

	if err := r.asignarRol(ctx, primero.ID, RolSuperadmin); err != nil {
		t.Fatalf("devolviendo el rol: %v", err)
	}
	if _, err := r.deshabilitar(ctx, segundo.ID); err != nil {
		t.Fatalf("deshabilitar teniendo otro superadmin habilitado: %v", err)
	}
}

// Un superadmin deshabilitado no cuenta. Si contara, quedarse sin nadie que
// pueda repartir permisos seria posible aun con la invariante puesta.
func TestIntegracionUnSuperadminDeshabilitadoNoSalvaAlUltimo(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	vivo := crearDePrueba(t, s, r, RolSuperadmin)
	apagado := crearDePrueba(t, s, r, RolSuperadmin)
	soloEsteSuperadmin(t, r, vivo.ID, apagado.ID)

	if _, err := r.pool.Exec(ctx, `update users set enabled = false where id = $1`, apagado.ID); err != nil {
		t.Fatalf("apagando al segundo superadmin: %v", err)
	}

	if err := r.quitarRol(ctx, vivo.ID, RolSuperadmin); !errors.Is(err, errUltimoSuperadmin) {
		t.Errorf("error = %v; un superadmin deshabilitado no puede contar como el que queda", err)
	}
}

// Deshabilitar dos veces es el mismo estado, no un conflicto. Un 409 aqui haria
// que un reintento —el caso normal cuando se corta la red— pareciera un error.
func TestIntegracionDeshabilitarDosVecesEsElMismoEstado(t *testing.T) {
	s, r := servicioReal(t)
	ctx := conActor(idDeQuienSiembra)

	u := crearDePrueba(t, s, r, RolStaff)

	if _, err := s.Deshabilitar(ctx, u.ID); err != nil {
		t.Fatalf("primera vez: %v", err)
	}
	otra, err := s.Deshabilitar(ctx, u.ID)
	if err != nil {
		t.Fatalf("segunda vez: %v", err)
	}
	if otra.Enabled {
		t.Error("la segunda respuesta dice que sigue habilitado")
	}
}

func TestIntegracionDeshabilitarAQuienNoExisteEs404(t *testing.T) {
	s, _ := servicioReal(t)

	_, err := s.Deshabilitar(conActor(idDeQuienSiembra), "00000000-0000-7000-8000-00000000dead")
	if err == nil {
		t.Fatal("no protesto")
	}
	if p := problemaDe(t, err); p.Status != 404 {
		t.Errorf("estado = %d, se esperaba 404", p.Status)
	}
}

// soloEsteSuperadmin deja fuera de la cuenta a cualquier otro superadmin de la
// base —el que sembro `cmd/seed`, por ejemplo— quitandoles el rol mientras dura
// la prueba, y se lo devuelve al terminar.
//
// Sin esto, "es el ultimo superadmin" depende de con que base te tocara correr,
// y la prueba pasaria en CI y fallaria en la maquina de quien ya sembro.
func soloEsteSuperadmin(t *testing.T, r *Repo, ids ...string) {
	t.Helper()
	ctx := context.Background()

	rows, err := r.pool.Query(ctx,
		`select ur.user_id from user_roles ur join roles ro on ro.id = ur.role_id
		  where ro.key = $1 and ur.user_id <> all($2::uuid[])`, RolSuperadmin, ids)
	if err != nil {
		t.Fatalf("buscando los otros superadmin: %v", err)
	}
	otros, err := pgx.CollectRows(rows, pgx.RowTo[string])
	rows.Close()
	if err != nil {
		t.Fatalf("buscando los otros superadmin: %v", err)
	}

	if len(otros) > 0 {
		if _, err := r.pool.Exec(ctx,
			`delete from user_roles
			  where role_id = (select id from roles where key = $1)
			    and user_id = any($2::uuid[])`, RolSuperadmin, otros); err != nil {
			t.Fatalf("apartando a los otros superadmin: %v", err)
		}
	}

	t.Cleanup(func() {
		for _, id := range otros {
			if err := r.asignarRol(context.Background(), id, RolSuperadmin); err != nil {
				t.Errorf("devolviendo el rol superadmin a %s: %v", id, err)
			}
		}
	})
}
