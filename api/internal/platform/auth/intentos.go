package auth

import (
	"sync"
	"time"
)

// El limite del login. Cinco intentos fallidos en quince minutos, y los dos
// contadores son independientes.
//
// Por correo Y por IP, no uno de los dos: solo por IP se saltea con NAT o con
// una botnet domestica, y solo por correo le deja a cualquiera bloquear la
// cuenta de un tercero a voluntad escribiendo mal la contrasena seis veces.
const (
	MaxIntentos    = 5
	VentanaIntento = 15 * time.Minute
)

// Intentos cuenta los fallos recientes. Vive en memoria, con barrido.
//
// **Lo que esto NO resuelve, escrito para que nadie lo descubra tarde:** el
// contador se pierde al reiniciar el proceso y no se comparte entre instancias,
// asi que con dos replicas detras de un balanceador el limite efectivo es el
// doble. Se acepta porque hoy hay una sola instancia y produccion esta sin
// decidir (Decision 011). El dia que haya dos, esto se muda a la base como el
// compare-and-set del refresh, por la misma razon que aquel: dos procesos no
// comparten memoria, pero si base.
type Intentos struct {
	mu     sync.Mutex
	fallos map[string][]time.Time
	ahora  func() time.Time
}

func NewIntentos() *Intentos {
	return &Intentos{fallos: make(map[string][]time.Time), ahora: time.Now}
}

// Permitido dice si se puede seguir, y cuanto falta si no.
//
// Se pregunta ANTES de tocar la contrasena: verificar argon2 y despues mirar el
// limite le regala al atacante el trabajo de CPU que el limite existe para
// negarle, y convierte el login en una forma comoda de tumbar el servidor.
func (i *Intentos) Permitido(claves ...string) (bool, time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()

	ahora := i.ahora()
	for _, k := range claves {
		vivos := i.vigentes(k, ahora)
		if len(vivos) >= MaxIntentos {
			// Cuanto falta para que el MAS VIEJO de los que cuentan salga de la
			// ventana, que es el instante en que vuelve a haber sitio.
			espera := VentanaIntento - ahora.Sub(vivos[0])
			if espera < time.Second {
				espera = time.Second
			}
			return false, espera
		}
	}
	return true, 0
}

// Fallo anota un intento fallido en cada clave.
func (i *Intentos) Fallo(claves ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	ahora := i.ahora()
	for _, k := range claves {
		i.fallos[k] = append(i.vigentes(k, ahora), ahora)
	}
}

// Exito limpia los contadores de quien acaba de entrar.
//
// Sin esto, cuatro intentos con la contrasena vieja mas uno bueno dejarian a
// alguien que YA demostro ser quien dice a un error de quedarse fuera quince
// minutos.
func (i *Intentos) Exito(claves ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for _, k := range claves {
		delete(i.fallos, k)
	}
}

// vigentes descarta lo que ya salio de la ventana. Se llama con el candado
// tomado.
//
// La lista esta ordenada por construccion —siempre se agrega al final, siempre
// con el ahora— asi que basta con cortar por el primero que sigue vigente.
func (i *Intentos) vigentes(clave string, ahora time.Time) []time.Time {
	sellos := i.fallos[clave]
	corte := ahora.Add(-VentanaIntento)

	n := 0
	for n < len(sellos) && !sellos[n].After(corte) {
		n++
	}
	return sellos[n:]
}

// Barrer tira las claves que ya no cuentan nada.
//
// Hace falta porque `vigentes` solo poda las claves que alguien vuelve a tocar:
// sin barrido, cada correo que se intento una vez se queda en el mapa para
// siempre y el limite de intentos se convierte en una fuga de memoria que
// cualquiera puede alimentar desde fuera.
func (i *Intentos) Barrer() {
	i.mu.Lock()
	defer i.mu.Unlock()

	ahora := i.ahora()
	for k := range i.fallos {
		if vivos := i.vigentes(k, ahora); len(vivos) == 0 {
			delete(i.fallos, k)
		} else {
			i.fallos[k] = vivos
		}
	}
}

// BarrerCada deja el barrido corriendo hasta que se cancele el contexto. Lo
// arranca `app`; el contexto es el del proceso, asi que la goroutine muere con
// el y no se queda colgada en las pruebas.
func (i *Intentos) BarrerCada(hecho <-chan struct{}, cada time.Duration) {
	t := time.NewTicker(cada)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-hecho:
				return
			case <-t.C:
				i.Barrer()
			}
		}
	}()
}

// ClaveEmail y ClaveIP separan los espacios de nombres de las dos cuentas.
//
// Sin el prefijo, una IP literal y un correo podrian colisionar en la misma
// clave del mapa. Es improbable y costaria un bloqueo inexplicable de rastrear.
func ClaveEmail(email string) string { return "email:" + email }
func ClaveIP(ip string) string       { return "ip:" + ip }
