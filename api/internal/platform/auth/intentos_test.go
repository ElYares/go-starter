package auth

import (
	"testing"
	"time"
)

func intentosConReloj(ahora *time.Time) *Intentos {
	i := NewIntentos()
	i.ahora = func() time.Time { return *ahora }
	return i
}

func TestLosPrimerosIntentosPasanYElSextoNo(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveEmail("ana@casa.com")

	for n := 1; n <= MaxIntentos; n++ {
		if ok, _ := i.Permitido(clave); !ok {
			t.Fatalf("el intento %d se bloqueo y no deberia", n)
		}
		i.Fallo(clave)
	}

	ok, espera := i.Permitido(clave)
	if ok {
		t.Fatal("el sexto intento paso")
	}
	if espera <= 0 || espera > VentanaIntento {
		t.Errorf("espera = %v, tiene que estar dentro de la ventana", espera)
	}
}

// La ventana es deslizante: pasada, se vuelve a poder. Un bloqueo permanente
// convertiria cinco errores de tecleo en una cuenta perdida.
func TestPasadaLaVentanaSeVuelveAPoder(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveEmail("ana@casa.com")

	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(clave)
	}
	if ok, _ := i.Permitido(clave); ok {
		t.Fatal("no se bloqueo")
	}

	reloj = reloj.Add(VentanaIntento + time.Second)
	if ok, _ := i.Permitido(clave); !ok {
		t.Fatal("sigue bloqueado despues de la ventana")
	}
}

// Los dos contadores son independientes, y las dos mitades importan: solo por
// IP se saltea con NAT, solo por correo deja bloquear a un tercero.
func TestLosContadoresPorCorreoYPorIpSonIndependientes(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)

	// Cinco fallos del mismo correo desde IPs distintas: el correo se agota.
	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(ClaveEmail("ana@casa.com"))
	}

	if ok, _ := i.Permitido(ClaveEmail("ana@casa.com"), ClaveIP("10.0.0.9")); ok {
		t.Error("el correo agotado paso desde una IP nueva")
	}
	if ok, _ := i.Permitido(ClaveEmail("otro@casa.com"), ClaveIP("10.0.0.9")); !ok {
		t.Error("otro correo desde la misma IP quedo bloqueado por el primero")
	}
}

func TestUnaIpAgotadaBloqueaAunqueElCorreoSeaNuevo(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)

	// El caso de la fuerza bruta: correos distintos desde la misma IP.
	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(ClaveIP("10.0.0.9"))
	}

	if ok, _ := i.Permitido(ClaveEmail("nuevo@casa.com"), ClaveIP("10.0.0.9")); ok {
		t.Error("la IP agotada paso probando un correo nuevo")
	}
}

// Entrar bien limpia el contador. Sin esto, cuatro intentos con la contrasena
// vieja mas uno bueno dejan a alguien que YA demostro ser quien dice a un error
// de quedarse fuera quince minutos.
func TestEntrarBienLimpiaLoQueSeLlevabaFallado(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveEmail("ana@casa.com")

	for n := 0; n < MaxIntentos-1; n++ {
		i.Fallo(clave)
	}
	i.Exito(clave)

	for n := 1; n <= MaxIntentos; n++ {
		if ok, _ := i.Permitido(clave); !ok {
			t.Fatalf("el intento %d se bloqueo despues de un exito", n)
		}
		i.Fallo(clave)
	}
}

// La espera que se devuelve es hasta que el MAS VIEJO salga de la ventana, que
// es cuando de verdad vuelve a haber sitio. Devolver la ventana entera mandaria
// a esperar de mas.
func TestLaEsperaCuentaDesdeElIntentoMasViejo(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveEmail("ana@casa.com")

	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(clave)
	}

	reloj = reloj.Add(10 * time.Minute)
	_, espera := i.Permitido(clave)

	if espera > 5*time.Minute+time.Second || espera < 4*time.Minute {
		t.Errorf("espera = %v; quedaban unos 5 minutos de la ventana", espera)
	}
}

// Nunca se devuelve una espera de cero: un "reintenta en 0 segundos" es un
// cliente reintentando de inmediato para recibir el mismo 429.
func TestLaEsperaNuncaEsCero(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveIP("10.0.0.9")

	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(clave)
	}

	// Justo en el borde: el mas viejo esta a un pelo de salir de la ventana.
	reloj = reloj.Add(VentanaIntento - time.Millisecond)
	if ok, espera := i.Permitido(clave); !ok && espera <= 0 {
		t.Errorf("espera = %v, tiene que ser positiva", espera)
	}
}

// Sin barrido, cada correo que se intento una vez se queda en el mapa para
// siempre: el limite de intentos se convierte en una fuga de memoria que
// cualquiera puede alimentar desde fuera mandando correos inventados.
func TestElBarridoTiraLoQueYaNoCuenta(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)

	for _, e := range []string{"a@x.com", "b@x.com", "c@x.com"} {
		i.Fallo(ClaveEmail(e))
	}
	if len(i.fallos) != 3 {
		t.Fatalf("claves = %d, se esperaban 3", len(i.fallos))
	}

	reloj = reloj.Add(VentanaIntento + time.Second)
	i.Barrer()

	if len(i.fallos) != 0 {
		t.Errorf("quedaron %d claves despues del barrido", len(i.fallos))
	}
}

// El barrido tiene que ocurrir SIN que nadie lo llame. La version anterior
// dependia de un `BarrerCada` que ningun codigo arrancaba, y las dos pruebas de
// arriba pasaban igual porque llaman a `Barrer` a mano: afirmaban que barrer
// funciona, no que se barre.
func TestLosFallosBarrenSolosLoQueYaNoCuenta(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)

	for _, e := range []string{"a@x.com", "b@x.com", "c@x.com"} {
		i.Fallo(ClaveEmail(e))
	}

	reloj = reloj.Add(VentanaIntento + time.Second)
	i.Fallo(ClaveEmail("nuevo@x.com"))

	if len(i.fallos) != 1 {
		t.Errorf("claves = %d; los fallos viejos tenian que irse con el siguiente fallo", len(i.fallos))
	}
}

// Y el barrido no se lleva lo que todavia cuenta, que seria regalar intentos.
func TestElBarridoNoTiraLoQueTodaviaCuenta(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)
	clave := ClaveEmail("ana@casa.com")

	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(clave)
	}

	reloj = reloj.Add(time.Minute)
	i.Barrer()

	if ok, _ := i.Permitido(clave); ok {
		t.Error("el barrido regalo intentos dentro de la ventana")
	}
}

// Las dos familias de clave no pueden colisionar. La prueba no compara las
// cadenas —eso solo afirma la forma— sino el EFECTO: agotar el contador de un
// correo que se llama igual que una IP no puede dejar sin intentos a esa IP.
//
// Comparar `ClaveEmail(x) != ClaveIP(x)` pasaria con los prefijos puestos, con
// uno solo puesto, y con cualquier par de prefijos distintos. Esto solo pasa si
// de verdad son dos cajones.
func TestUnCorreoNoGastaElContadorDeUnaIpQueSeLlameIgual(t *testing.T) {
	reloj := time.Now()
	i := intentosConReloj(&reloj)

	// Un correo cuyo texto es exactamente una IP. Rebuscado a proposito: es la
	// unica forma de que dos espacios de nombres sin separar se toquen.
	for n := 0; n < MaxIntentos; n++ {
		i.Fallo(ClaveEmail("10.0.0.9"))
	}

	if ok, _ := i.Permitido(ClaveIP("10.0.0.9")); !ok {
		t.Error("los fallos de un correo agotaron el contador de la IP homonima")
	}
}
