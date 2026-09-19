package identity

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elyares/go-starter/api/internal/platform/ids"
)

// Lo del dashboard de cuentas que solo existe en la base: la transaccion de
// deshabilitar, la auditoria, la agregacion de roles, el `citext` al editar y
// la clave foranea al dar un rol. Mismas reglas que repo_integracion_test.go:
// se saltan sin DATABASE_URL y limpian lo que crean.

const (
	idDeQuienEdita       = "3f2a1b0c-9d8e-7000-8000-0a1b2c3d4e60"
	idDeQuienDeshabilita = "3f2a1b0c-9d8e-7000-8000-0a1b2c3d4e61"
	idDeQuienHabilita    = "3f2a1b0c-9d8e-7000-8000-0a1b2c3d4e62"
)

func sesionGuardada(t *testing.T, r *Repo, userID string) string {
	t.Helper()
	id, err := ids.NewString()
	if err != nil {
		t.Fatalf("ids: %v", err)
	}
	if err := r.guardarRefresh(context.Background(), SesionNueva{
		ID: id, UserID: userID, TokenHash: []byte(id), ExpiraEn: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("guardarRefresh: %v", err)
	}
	return id
}

func sesionesVivas(t *testing.T, r *Repo, userID string) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`select count(*) from refresh_tokens where user_id = $1 and revoked_at is null`, userID).Scan(&n); err != nil {
		t.Fatalf("contando sesiones: %v", err)
	}
	return n
}

func TestIntegracionDeshabilitarRevocaTodasSusSesiones(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolStaff)
	otra := crearDePrueba(t, s, r, RolStaff)
	sesionGuardada(t, r, u.ID)
	sesionGuardada(t, r, u.ID)
	sesionGuardada(t, r, otra.ID)

	if _, err := s.DeshabilitarCuenta(conActor(idDeQuienDeshabilita), u.ID); err != nil {
		t.Fatalf("DeshabilitarCuenta: %v", err)
	}

	if n := sesionesVivas(t, r, u.ID); n != 0 {
		t.Errorf("quedaron %d sesiones vivas de la cuenta deshabilitada", n)
	}
	// Y solo las suyas: una revocacion sin el WHERE de user_id sacaria a todo
	// el mundo, y la prueba de arriba seguiria pasando.
	if n := sesionesVivas(t, r, otra.ID); n != 1 {
		t.Errorf("la otra cuenta tiene %d sesiones vivas, se esperaba 1", n)
	}
}

// Cuando la invariante rechaza, no se revoca nada. Una revocacion que corriera
// antes del update, o sin mirar si escribio, dejaria al ultimo superadmin
// habilitado pero fuera de todas sus sesiones.
func TestIntegracionUnaDeshabilitacionRechazadaNoRevocaNada(t *testing.T) {
	s, r := servicioReal(t)
	ctx := context.Background()

	// Se deshabilitan temporalmente los demas superadmins para que este sea el
	// ultimo. La limpieza los devuelve como estaban.
	rows, err := r.pool.Query(ctx, `update users u set enabled = false
		  from user_roles ur join roles ro on ro.id = ur.role_id
		 where ur.user_id = u.id and ro.key = 'superadmin' and u.enabled
		returning u.id`)
	if err != nil {
		t.Fatalf("apartando superadmins: %v", err)
	}
	var apartados []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		apartados = append(apartados, id)
	}
	rows.Close()
	t.Cleanup(func() {
		_, _ = r.pool.Exec(context.Background(), `update users set enabled = true where id = any($1)`, apartados)
	})

	super := crearDePrueba(t, s, r, RolSuperadmin)
	sesionGuardada(t, r, super.ID)

	_, err = s.DeshabilitarCuenta(conActor(idDeQuienDeshabilita), super.ID)
	if p := problemaDe(t, err); p.Status != 409 {
		t.Fatalf("estado = %d, se esperaba 409", p.Status)
	}
	if n := sesionesVivas(t, r, super.ID); n != 1 {
		t.Errorf("el ultimo superadmin se quedo con %d sesiones vivas; el rechazo no puede revocar", n)
	}
}

// La auditoria la llena platform/audit leyendo el actor del contexto. Ninguna
// de las cuatro operaciones la asigna a mano, y esta prueba es la que lo
// comprueba en la fila.
func TestIntegracionLaAuditoriaDeUnaCuentaSeLlenaSola(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	porQuien := func() (creado, editado string) {
		t.Helper()
		if err := r.pool.QueryRow(context.Background(),
			`select created_by::text, updated_by::text from users where id = $1`, u.ID).Scan(&creado, &editado); err != nil {
			t.Fatalf("leyendo auditoria: %v", err)
		}
		return
	}

	if c, e := porQuien(); c != idDeQuienSiembra || e != idDeQuienSiembra {
		t.Errorf("al crear: created_by=%s updated_by=%s", c, e)
	}

	pasos := []struct {
		quien string
		hacer func(ctx context.Context) error
	}{
		{idDeQuienEdita, func(ctx context.Context) error {
			_, err := s.Editar(ctx, u.ID, 1, ModificacionDeUsuario{Email: u.Email, DisplayName: "Editada"})
			return err
		}},
		{idDeQuienDeshabilita, func(ctx context.Context) error {
			_, err := s.DeshabilitarCuenta(ctx, u.ID)
			return err
		}},
		{idDeQuienHabilita, func(ctx context.Context) error {
			_, err := s.Habilitar(ctx, u.ID)
			return err
		}},
	}
	for _, p := range pasos {
		if err := p.hacer(conActor(p.quien)); err != nil {
			t.Fatalf("%s: %v", p.quien, err)
		}
		c, e := porQuien()
		if c != idDeQuienSiembra {
			t.Errorf("created_by cambio a %s", c)
		}
		if e != p.quien {
			t.Errorf("updated_by = %s, se esperaba %s", e, p.quien)
		}
	}
}

func TestIntegracionEditarConUnaVersionViejaEs409YNoEscribe(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	if _, err := s.Editar(conActor(idDeQuienEdita), u.ID, 1, ModificacionDeUsuario{Email: u.Email, DisplayName: "Primera"}); err != nil {
		t.Fatalf("primer guardado: %v", err)
	}
	_, err := s.Editar(conActor(idDeQuienEdita), u.ID, 1, ModificacionDeUsuario{Email: u.Email, DisplayName: "Pisada"})
	if p := problemaDe(t, err); p.Status != 409 {
		t.Fatalf("estado = %d, se esperaba 409", p.Status)
	}

	leida, err := s.Cuenta(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("Cuenta: %v", err)
	}
	if leida.DisplayName != "Primera" || leida.Version != 2 {
		t.Errorf("cuenta = %+v; el guardado viejo escribio", leida.Usuario)
	}
}

func TestIntegracionEditarUnaCuentaQueNoExisteEs404(t *testing.T) {
	s, _ := servicioReal(t)

	_, err := s.Editar(conActor(idDeQuienEdita), "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", 1,
		ModificacionDeUsuario{Email: "nadie@casa.com", DisplayName: "Nadie"})
	if p := problemaDe(t, err); p.Status != 404 {
		t.Fatalf("estado = %d, se esperaba 404", p.Status)
	}
}

// El `citext` tambien vale al editar: ponerle a una cuenta el correo de otra
// en otras mayusculas es la misma colision que al crear.
func TestIntegracionEditarAlCorreoDeOtraEnMayusculasEs409(t *testing.T) {
	s, r := servicioReal(t)
	una := crearDePrueba(t, s, r)
	otra := crearDePrueba(t, s, r)

	_, err := s.Editar(conActor(idDeQuienEdita), otra.ID, 1,
		ModificacionDeUsuario{Email: strings.ToUpper(una.Email), DisplayName: "Otra"})
	if p := problemaDe(t, err); p.Status != 409 {
		t.Fatalf("estado = %d, se esperaba 409", p.Status)
	}
}

func TestIntegracionElListadoTraeLosRolesDeCadaCuenta(t *testing.T) {
	s, r := servicioReal(t)
	conDos := crearDePrueba(t, s, r, RolStaff, RolAdmin)
	sinRoles := crearDePrueba(t, s, r)

	pagina, err := s.Cuentas(context.Background(), url.Values{"size": {"100"}, "sort": {"createdAt,desc"}})
	if err != nil {
		t.Fatalf("Cuentas: %v", err)
	}

	porID := map[string]UsuarioConRoles{}
	for _, c := range pagina.Content {
		porID[c.ID] = c
	}
	if got := porID[conDos.ID].Roles; !slices.Equal(got, []string{RolAdmin, RolStaff}) {
		t.Errorf("roles = %v, se esperaban [admin staff] ordenados", got)
	}
	// `{}` y no `{NULL}`: un left join sin coincidencias agrega un nulo si no
	// se filtra, y la cuenta saldria con un rol vacio.
	if c, ok := porID[sinRoles.ID]; !ok || c.Roles == nil || len(c.Roles) != 0 {
		t.Errorf("la cuenta sin roles salio con %#v", c.Roles)
	}
}

func TestIntegracionElListadoFiltraPorHabilitada(t *testing.T) {
	s, r := servicioReal(t)
	apagada := crearDePrueba(t, s, r)
	if _, err := s.Deshabilitar(conActor(idDeQuienDeshabilita), apagada.ID); err != nil {
		t.Fatalf("Deshabilitar: %v", err)
	}

	pagina, err := s.Cuentas(context.Background(), url.Values{"enabled": {"false"}, "size": {"100"}})
	if err != nil {
		t.Fatalf("Cuentas: %v", err)
	}
	encontrada := false
	for _, c := range pagina.Content {
		if c.Enabled {
			t.Errorf("%s esta habilitada y el filtro pedia las que no", c.Email)
		}
		encontrada = encontrada || c.ID == apagada.ID
	}
	if !encontrada {
		t.Error("la cuenta deshabilitada no salio en el filtro")
	}
}

// La clave foranea de `user_roles`: darle un rol a una cuenta que no existe es
// un 404, no el 500 de una violacion sin traducir.
func TestIntegracionDarUnRolAUnaCuentaQueNoExisteEs404(t *testing.T) {
	s, _ := servicioReal(t)

	err := s.DarRol(context.Background(), "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", RolStaff)
	if p := problemaDe(t, err); p.Status != 404 {
		t.Fatalf("estado = %d, se esperaba 404", p.Status)
	}
}

func TestIntegracionDarUnRolQueNoExisteEs404(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	err := s.DarRol(context.Background(), u.ID, "inventado")
	if p := problemaDe(t, err); p.Status != 404 {
		t.Fatalf("estado = %d, se esperaba 404", p.Status)
	}
}

// La pantalla de roles tiene que poder avisar de lo que concede cada uno. El
// catalogo sembrado es el de los modulos de verdad, asi que se comprueba contra
// lo que declara identity.
func TestIntegracionLosRolesTraenSusPermisosConLosSensiblesMarcados(t *testing.T) {
	s, _ := servicioReal(t)

	pagina, err := s.RolesConPermisos(context.Background(), url.Values{})
	if err != nil {
		t.Fatalf("RolesConPermisos: %v", err)
	}

	porClave := map[string]RolConPermisos{}
	for _, rol := range pagina.Content {
		porClave[rol.Key] = rol
	}
	super, ok := porClave[RolSuperadmin]
	if !ok {
		t.Fatalf("no salio el superadmin: %+v", pagina.Content)
	}

	sensible := false
	for _, p := range super.Permisos {
		if p.Key == "identity.role.assign" {
			sensible = p.Sensitive
		}
	}
	if !sensible {
		t.Errorf("identity.role.assign no salio marcado como sensible en el superadmin: %+v", super.Permisos)
	}
	for _, p := range porClave[RolAdmin].Permisos {
		if p.Sensitive {
			t.Errorf("el admin concede %s, que es sensible", p.Key)
		}
	}
}

// El criterio del 401 inmediato, contra la base: una cuenta deshabilitada deja
// de resolver actor aunque su `at` siga vigente.
func TestIntegracionUnaCuentaDeshabilitadaNoResuelveActor(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolStaff)

	if _, err := s.Deshabilitar(conActor(idDeQuienDeshabilita), u.ID); err != nil {
		t.Fatalf("Deshabilitar: %v", err)
	}
	if _, err := s.Actor(context.Background(), u.ID); !errors.Is(err, errSinAcceso) {
		t.Errorf("Actor = %v, se esperaba errSinAcceso", err)
	}
}

// Habilitar una cuenta habilitada es el mismo estado: ni sube la version ni
// toca la auditoria. Si subiera, el editor que la tenia abierta recibiria un
// 409 por un cambio que no cambio nada.
func TestIntegracionHabilitarUnaCuentaHabilitadaNoLaToca(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r)

	h, err := s.Habilitar(conActor(idDeQuienHabilita), u.ID)
	if err != nil {
		t.Fatalf("Habilitar: %v", err)
	}
	if h.Version != u.Version {
		t.Errorf("version = %d, se esperaba %d: no cambio nada", h.Version, u.Version)
	}
	if h.UpdatedBy == nil || *h.UpdatedBy != idDeQuienSiembra {
		t.Errorf("updated_by = %v; habilitar sin cambio no deja rastro", h.UpdatedBy)
	}
}
