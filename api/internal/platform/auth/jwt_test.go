package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const llaveDePrueba = "una-llave-de-prueba-que-no-vale-fuera-de-aqui"

func firmanteDePrueba(t *testing.T) *Firmante {
	t.Helper()
	f, err := NewFirmante(llaveDePrueba)
	if err != nil {
		t.Fatalf("NewFirmante: %v", err)
	}
	return f
}

func TestUnTokenFirmadoSeVerificaYDevuelveLoQueSeGuardo(t *testing.T) {
	f := firmanteDePrueba(t)

	token, err := f.Firmar("u-1", []string{"admin", "staff"})
	if err != nil {
		t.Fatalf("Firmar: %v", err)
	}

	c, err := f.Verificar(token)
	if err != nil {
		t.Fatalf("Verificar: %v", err)
	}
	if c.Sub != "u-1" {
		t.Errorf("sub = %q, se esperaba u-1", c.Sub)
	}
	if strings.Join(c.Roles, ",") != "admin,staff" {
		t.Errorf("roles = %v", c.Roles)
	}
}

// Sin llave no hay firma que valga. Un firmante con llave vacia produce tokens
// que cualquiera reproduce, asi que no se construye.
func TestUnFirmanteSinLlaveNoSeConstruye(t *testing.T) {
	for _, llave := range []string{"", "   ", "\t\n"} {
		if _, err := NewFirmante(llave); err == nil {
			t.Errorf("se construyo un firmante con la llave %q", llave)
		}
	}
}

// La prueba que justifica escribir esto a mano: `alg: none` es el fallo clasico
// de las implementaciones de JWT. Un verificador que lee el `alg` del propio
// token para decidir como verificar acepta un token sin firma.
func TestUnTokenConAlgNoneSeRechaza(t *testing.T) {
	cuerpo, _ := json.Marshal(Claims{Sub: "u-1", Exp: time.Now().Add(time.Hour).Unix()})
	token := codificar([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + codificar(cuerpo) + "."

	if _, err := firmanteDePrueba(t).Verificar(token); err == nil {
		t.Fatal("se acepto un token sin firma con alg=none")
	}
}

// El otro fallo clasico: cambiar HS256 por otro algoritmo. Aqui la cabecera se
// compara literal, asi que cualquier cambio —el algoritmo, un campo de mas, o
// los mismos campos en otro orden— cae por el mismo sitio.
func TestUnTokenConOtroAlgoritmoSeRechaza(t *testing.T) {
	f := firmanteDePrueba(t)
	valido, _ := f.Firmar("u-1", nil)
	partes := strings.Split(valido, ".")

	for _, cabecera := range []string{
		`{"alg":"RS256","typ":"JWT"}`,
		`{"alg":"HS512","typ":"JWT"}`,
		`{"typ":"JWT","alg":"HS256"}`,
		`{"alg":"HS256","typ":"JWT","kid":"1"}`,
	} {
		token := codificar([]byte(cabecera)) + "." + partes[1] + "." + partes[2]
		if _, err := f.Verificar(token); err != ErrAlgoritmo {
			t.Errorf("cabecera %s: err = %v, se esperaba ErrAlgoritmo", cabecera, err)
		}
	}
}

// Un token firmado con OTRA llave no vale, que es lo que impide que alguien
// emita sesiones desde fuera.
func TestUnTokenFirmadoConOtraLlaveNoVale(t *testing.T) {
	otro, err := NewFirmante("otra-llave-completamente-distinta")
	if err != nil {
		t.Fatalf("NewFirmante: %v", err)
	}

	token, _ := otro.Firmar("u-1", nil)
	if _, err := firmanteDePrueba(t).Verificar(token); err != ErrFirmaInvalida {
		t.Fatalf("err = %v, se esperaba ErrFirmaInvalida", err)
	}
}

// Tocar el contenido invalida la firma. Es la prueba de que los claims no se
// pueden editar en transito: sin ella, cualquiera se agrega el rol superadmin.
func TestManipularLosClaimsInvalidaLaFirma(t *testing.T) {
	f := firmanteDePrueba(t)
	token, _ := f.Firmar("u-1", []string{"viewer"})
	partes := strings.Split(token, ".")

	falso, _ := json.Marshal(Claims{
		Sub:   "u-1",
		Roles: []string{"superadmin"},
		Exp:   time.Now().Add(time.Hour).Unix(),
	})

	manipulado := partes[0] + "." + codificar(falso) + "." + partes[2]
	if _, err := f.Verificar(manipulado); err != ErrFirmaInvalida {
		t.Fatalf("err = %v, se esperaba ErrFirmaInvalida", err)
	}
}

func TestUnTokenExpiradoSeRechaza(t *testing.T) {
	f := firmanteDePrueba(t)

	// Se firma con el reloj atrasado una hora, asi que nace vencido. Mover el
	// reloj y no dormir: una prueba que espera 15 minutos no la corre nadie.
	f.ahora = func() time.Time { return time.Now().Add(-time.Hour) }
	token, _ := f.Firmar("u-1", nil)

	f.ahora = time.Now
	if _, err := f.Verificar(token); err != ErrExpirado {
		t.Fatalf("err = %v, se esperaba ErrExpirado", err)
	}
}

// Un token sin `exp` es un token eterno. El cero de Go lo deja en 1970, y la
// comparacion lo rechaza — pero hay que comprobarlo, porque el dia que alguien
// cambie el `!Before` por un `After` con un cero por medio esto pasa a valer.
func TestUnTokenSinCaducidadNoEsEterno(t *testing.T) {
	f := firmanteDePrueba(t)

	cuerpo, _ := json.Marshal(map[string]any{"sub": "u-1", "roles": []string{}})
	firmado := cabeceraCodificada + "." + codificar(cuerpo)
	token := firmado + "." + codificar(f.firma(firmado))

	if _, err := f.Verificar(token); err != ErrExpirado {
		t.Fatalf("err = %v, se esperaba ErrExpirado: un token sin exp no caduca nunca", err)
	}
}

// Un token valido pero sin sujeto no identifica a nadie. Dejarlo pasar pondria
// un actor con id vacio en el contexto, y las consultas filtradas por
// propiedad devolverian las filas de nadie... o las de todos.
func TestUnTokenSinSujetoSeRechaza(t *testing.T) {
	f := firmanteDePrueba(t)

	if _, err := f.Firmar("", nil); err != ErrSinSujeto {
		t.Fatalf("al firmar: err = %v, se esperaba ErrSinSujeto", err)
	}

	cuerpo, _ := json.Marshal(Claims{Exp: time.Now().Add(time.Hour).Unix()})
	firmado := cabeceraCodificada + "." + codificar(cuerpo)
	if _, err := f.Verificar(firmado + "." + codificar(f.firma(firmado))); err != ErrSinSujeto {
		t.Fatalf("al verificar: err = %v, se esperaba ErrSinSujeto", err)
	}
}

func TestUnTokenMalFormadoNoRevientaNada(t *testing.T) {
	f := firmanteDePrueba(t)

	for _, token := range []string{
		"", ".", "..", "a.b", "a.b.c.d",
		"a.b.c",
		cabeceraCodificada + ".no-es-base64!.zzz",
		cabeceraCodificada + "." + codificar([]byte("{no es json")) + ".zzz",
	} {
		if _, err := f.Verificar(token); err == nil {
			t.Errorf("se acepto el token mal formado %q", token)
		}
	}
}

// El JWT viaja en una cookie, y un `=` de relleno obligaria a escaparlo. Por
// eso base64url SIN relleno, que ademas es lo que dice el RFC 7515.
func TestElTokenNoLlevaRellenoBase64(t *testing.T) {
	token, _ := firmanteDePrueba(t).Firmar("u-1", []string{"admin"})

	if strings.Contains(token, "=") {
		t.Errorf("el token lleva relleno: %q", token)
	}
	for _, parte := range strings.Split(token, ".") {
		if _, err := base64.RawURLEncoding.DecodeString(parte); err != nil {
			t.Errorf("la parte %q no es base64url sin relleno: %v", parte, err)
		}
	}
}

// Firmar sin roles tiene que producir `[]` y no `null`: al otro lado, un null
// donde se espera una lista es un `.includes()` que revienta.
func TestLosRolesVaciosSalenComoListaYNoComoNull(t *testing.T) {
	f := firmanteDePrueba(t)
	token, _ := f.Firmar("u-1", nil)

	cuerpo, err := decodificar(strings.Split(token, ".")[1])
	if err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if !strings.Contains(string(cuerpo), `"roles":[]`) {
		t.Errorf("el cuerpo no lleva una lista vacia de roles: %s", cuerpo)
	}
}
