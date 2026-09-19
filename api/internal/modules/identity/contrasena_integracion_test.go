package identity

import (
	"context"
	"testing"

	"github.com/elyares/go-starter/api/internal/platform/auth"
)

// Lo de la contrasena temporal que solo existe en la base: la regla de poder en
// el WHERE, la solicitud unica por cuenta, y que asignar revoque y atienda en la
// misma transaccion. Usan el catalogo sembrado de verdad: el admin tiene los
// permisos no sensibles y el superadmin todos.

// firmanteDePrueba: CambiarContrasena emite una sesion, y servicioReal arma el
// service sin firmante porque las demas pruebas de integracion no lo usan.
func firmanteDePrueba(t *testing.T) *auth.Firmante {
	t.Helper()
	f, err := auth.NewFirmante("llave-de-prueba-de-integracion")
	if err != nil {
		t.Fatalf("NewFirmante: %v", err)
	}
	return f
}

func pendientesDe(t *testing.T, r *Repo, userID string) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`select count(*) from password_reset_requests where user_id = $1 and resolved_at is null`, userID).Scan(&n); err != nil {
		t.Fatalf("contando solicitudes: %v", err)
	}
	return n
}

func hashGuardado(t *testing.T, r *Repo, userID string) string {
	t.Helper()
	var h string
	if err := r.pool.QueryRow(context.Background(), `select password_hash from users where id = $1`, userID).Scan(&h); err != nil {
		t.Fatalf("leyendo el hash: %v", err)
	}
	return h
}

func TestIntegracionPedirDosVecesDejaUnaSolaSolicitud(t *testing.T) {
	s, r := servicioReal(t)
	u := crearDePrueba(t, s, r, RolStaff)

	for range 3 {
		if err := s.PedirContrasena(context.Background(), u.Email, "203.0.113.7", "prueba"); err != nil {
			t.Fatalf("PedirContrasena: %v", err)
		}
	}
	if n := pendientesDe(t, r, u.ID); n != 1 {
		t.Errorf("pendientes = %d, se esperaba 1", n)
	}
}

// Una cuenta deshabilitada o un correo que no existe no dejan nada, y no
// fallan: el pedido responde igual.
func TestIntegracionPedirPorUnaCuentaApagadaONoRegistradaNoDejaNada(t *testing.T) {
	s, r := servicioReal(t)
	apagada := crearDePrueba(t, s, r)
	if _, err := s.Deshabilitar(conActor(idDeQuienDeshabilita), apagada.ID); err != nil {
		t.Fatalf("Deshabilitar: %v", err)
	}

	if err := s.PedirContrasena(context.Background(), apagada.Email, "", ""); err != nil {
		t.Fatalf("apagada: %v", err)
	}
	if err := s.PedirContrasena(context.Background(), "prueba.integracion.nadie@casa.com", "", ""); err != nil {
		t.Fatalf("nadie: %v", err)
	}
	if n := pendientesDe(t, r, apagada.ID); n != 0 {
		t.Errorf("la cuenta deshabilitada dejo %d solicitudes", n)
	}
}

func TestIntegracionElAdminAsignaAStaffYQuedaTodoHecho(t *testing.T) {
	s, r := servicioReal(t)
	admin := crearDePrueba(t, s, r, RolAdmin)
	staff := crearDePrueba(t, s, r, RolStaff)
	sesionGuardada(t, r, staff.ID)
	if err := s.PedirContrasena(context.Background(), staff.Email, "", ""); err != nil {
		t.Fatalf("PedirContrasena: %v", err)
	}

	if err := s.AsignarContrasena(conActor(admin.ID), admin.ID, staff.ID, "una temporal bien larga"); err != nil {
		t.Fatalf("AsignarContrasena: %v", err)
	}

	leida, err := s.Cuenta(context.Background(), staff.ID)
	if err != nil {
		t.Fatalf("Cuenta: %v", err)
	}
	if !leida.MustChangePassword {
		t.Error("no quedo como temporal")
	}
	if leida.UpdatedBy == nil || *leida.UpdatedBy != admin.ID {
		t.Errorf("updated_by = %v, se esperaba el admin", leida.UpdatedBy)
	}
	if n := sesionesVivas(t, r, staff.ID); n != 0 {
		t.Errorf("quedaron %d sesiones vivas", n)
	}
	if n := pendientesDe(t, r, staff.ID); n != 0 {
		t.Errorf("quedaron %d solicitudes pendientes", n)
	}
	var atendidaPor string
	if err := r.pool.QueryRow(context.Background(),
		`select resolved_by::text from password_reset_requests where user_id = $1`, staff.ID).Scan(&atendidaPor); err != nil || atendidaPor != admin.ID {
		t.Errorf("resolved_by = %q (%v), se esperaba el admin", atendidaPor, err)
	}
}

// El criterio que hace seguro darle el permiso al admin: no puede asignarle la
// contrasena a quien tiene mas poder que el. Si pudiera, entraria como el
// superadmin y se daria todo.
func TestIntegracionElAdminNoPuedeConElSuperadmin(t *testing.T) {
	s, r := servicioReal(t)
	admin := crearDePrueba(t, s, r, RolAdmin)
	super := crearDePrueba(t, s, r, RolSuperadmin)
	sesionGuardada(t, r, super.ID)
	hash := hashGuardado(t, r, super.ID)

	err := s.AsignarContrasena(conActor(admin.ID), admin.ID, super.ID, "una temporal bien larga")

	if p := problemaDe(t, err); p.Status != 403 {
		t.Fatalf("estado = %d, se esperaba 403", p.Status)
	}
	if hashGuardado(t, r, super.ID) != hash {
		t.Error("el hash del superadmin cambio")
	}
	if n := sesionesVivas(t, r, super.ID); n != 1 {
		t.Errorf("el rechazo revoco sesiones: vivas = %d", n)
	}
}

// Y al reves si: el superadmin tiene todo, asi que nadie tiene mas que el.
func TestIntegracionElSuperadminPuedeConCualquiera(t *testing.T) {
	s, r := servicioReal(t)
	super := crearDePrueba(t, s, r, RolSuperadmin)
	otroSuper := crearDePrueba(t, s, r, RolSuperadmin)

	if err := s.AsignarContrasena(conActor(super.ID), super.ID, otroSuper.ID, "una temporal bien larga"); err != nil {
		t.Errorf("AsignarContrasena: %v", err)
	}
}

// Un rol de mas en la cuenta la pone fuera de alcance, aunque el resto
// coincida: la regla compara permisos, no roles.
func TestIntegracionUnaCuentaConUnPermisoDeMasQuedaFueraDeAlcance(t *testing.T) {
	s, r := servicioReal(t)
	admin := crearDePrueba(t, s, r, RolAdmin)
	adminYSuper := crearDePrueba(t, s, r, RolAdmin, RolSuperadmin)

	err := s.AsignarContrasena(conActor(admin.ID), admin.ID, adminYSuper.ID, "una temporal bien larga")
	if p := problemaDe(t, err); p.Status != 403 {
		t.Fatalf("estado = %d, se esperaba 403", p.Status)
	}
}

func TestIntegracionAsignarAUnaCuentaQueNoExisteEs404(t *testing.T) {
	s, r := servicioReal(t)
	super := crearDePrueba(t, s, r, RolSuperadmin)

	err := s.AsignarContrasena(conActor(super.ID), super.ID, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", "una temporal bien larga")
	if p := problemaDe(t, err); p.Status != 404 {
		t.Fatalf("estado = %d, se esperaba 404", p.Status)
	}
}

func TestIntegracionConContrasenaTemporalNoHayPermisosHastaCambiarla(t *testing.T) {
	s, r := servicioReal(t)
	super := crearDePrueba(t, s, r, RolSuperadmin)
	admin := crearDePrueba(t, s, r, RolAdmin)
	if err := s.AsignarContrasena(conActor(super.ID), super.ID, admin.ID, "una temporal bien larga"); err != nil {
		t.Fatalf("AsignarContrasena: %v", err)
	}

	actor, err := s.Actor(context.Background(), admin.ID)
	if err != nil || len(actor.Permissions) != 0 {
		t.Fatalf("con temporal: actor = %+v, err = %v", actor, err)
	}

	sesionGuardada(t, r, admin.ID)
	s.firmante = firmanteDePrueba(t)
	if _, err := s.CambiarContrasena(conActor(admin.ID), admin.ID, "una temporal bien larga", "la mia nueva y larga", "", ""); err != nil {
		t.Fatalf("CambiarContrasena: %v", err)
	}

	actor, err = s.Actor(context.Background(), admin.ID)
	if err != nil || !actor.Can("content.page.read") {
		t.Errorf("tras cambiarla: actor = %+v, err = %v", actor, err)
	}
	// La sesion vieja revocada; la nueva la emitio el cambio.
	if n := sesionesVivas(t, r, admin.ID); n != 1 {
		t.Errorf("sesiones vivas = %d, se esperaba solo la nueva", n)
	}
	if _, err := s.Autenticar(context.Background(), admin.Email, "la mia nueva y larga"); err != nil {
		t.Errorf("no se entra con la nueva: %v", err)
	}
	if _, err := s.Autenticar(context.Background(), admin.Email, "una temporal bien larga"); err == nil {
		t.Error("la temporal sigue sirviendo")
	}
}
