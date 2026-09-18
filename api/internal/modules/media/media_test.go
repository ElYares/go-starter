package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/elyares/go-starter/api/internal/platform/httpx"
	"github.com/elyares/go-starter/api/internal/platform/observ"
	"github.com/elyares/go-starter/api/internal/platform/rbac"
	"github.com/elyares/go-starter/api/internal/platform/storage"
)

const idDeAna = "3f1c9b2e-7d5a-4c81-9e0f-2a6b8c4d1e33"

// repoFalso reemplaza a Postgres con un mapa por hash, que es exactamente lo
// que hace el indice unico: la misma imagen, la misma fila.
type repoFalso struct {
	porSum  map[string]registro
	inserts int
}

func nuevoRepo() *repoFalso { return &repoFalso{porSum: map[string]registro{}} }

func (r *repoFalso) obtener(_ context.Context, id uuid.UUID) (registro, error) {
	for _, reg := range r.porSum {
		if reg.Id == id {
			return reg, nil
		}
	}
	return registro{}, errNoExiste
}

func (r *repoFalso) porHash(_ context.Context, sum []byte) (registro, error) {
	if reg, ok := r.porSum[hex.EncodeToString(sum)]; ok {
		return reg, nil
	}
	return registro{}, errNoExiste
}

func (r *repoFalso) insertar(_ context.Context, n nuevo) (registro, bool, error) {
	clave := hex.EncodeToString(n.sha256)
	if reg, ok := r.porSum[clave]; ok {
		return reg, false, nil
	}
	r.inserts++
	id := uuid.New()
	reg := registro{
		Medio: Medio{
			Id: id, Url: urlPublica(id), Mime: MedioMime(n.mime), SizeBytes: n.tamano,
			Width: n.ancho, Height: n.alto, Sha256: clave, OriginalName: n.nombre,
			CreatedAt: time.Now(),
		},
		llave: n.llave,
	}
	r.porSum[clave] = reg
	return reg, true, nil
}

type banco struct {
	repo  *repoFalso
	raiz  string
	store *storage.Local
}

func nuevoBanco(t *testing.T) *banco {
	t.Helper()
	raiz := t.TempDir()
	st, err := storage.NewLocal(raiz)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return &banco{repo: nuevoRepo(), raiz: raiz, store: st}
}

func (b *banco) servidor(actor *rbac.Actor) http.Handler {
	m := &Module{svc: &Service{repo: b.repo, store: b.store}}
	r := httpx.NewRouter()
	m.Routes(r)

	inyectar := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if actor != nil {
				req = req.WithContext(rbac.WithActor(req.Context(), *actor))
			}
			next.ServeHTTP(w, req)
		})
	}
	return observ.Chain(r.Handler(), observ.TraceID, inyectar)
}

// archivos cuenta lo guardado con llave y lo que quedo en lo provisional. Son
// las dos cosas que una subida rechazada no puede dejar.
func (b *banco) archivos(t *testing.T) (guardados, provisionales int) {
	t.Helper()
	err := filepath.WalkDir(b.raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.Contains(ruta, string(filepath.Separator)+".tmp"+string(filepath.Separator)) {
			provisionales++
		} else {
			guardados++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo el almacenamiento: %v", err)
	}
	return guardados, provisionales
}

func con(permisos ...string) *rbac.Actor {
	return &rbac.Actor{ID: idDeAna, Permissions: permisos}
}

func quienSube() *rbac.Actor { return con("media.read", "media.upload") }

// subida arma el multipart a mano: lo que se prueba es lo que manda un cliente.
type campo struct {
	nombre    string
	archivo   string
	tipo      string
	contenido []byte
}

func subir(t *testing.T, b *banco, actor *rbac.Actor, campos ...campo) *httptest.ResponseRecorder {
	t.Helper()
	var cuerpo bytes.Buffer
	mw := multipart.NewWriter(&cuerpo)
	for _, c := range campos {
		h := make(map[string][]string)
		disp := `form-data; name="` + c.nombre + `"`
		if c.archivo != "" {
			disp += `; filename="` + c.archivo + `"`
		}
		h["Content-Disposition"] = []string{disp}
		if c.tipo != "" {
			h["Content-Type"] = []string{c.tipo}
		}
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(c.contenido); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/media", &cuerpo)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	b.servidor(actor).ServeHTTP(rec, req)
	return rec
}

func archivo(nombre string, contenido []byte) campo {
	return campo{nombre: "file", archivo: nombre, tipo: "image/png", contenido: contenido}
}

func pedir(t *testing.T, b *banco, actor *rbac.Actor, ruta string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	b.servidor(actor).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
	return rec
}

func medio(t *testing.T, rec *httptest.ResponseRecorder) Medio {
	t.Helper()
	var m Medio
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("el cuerpo no es un Medio: %q", rec.Body.String())
	}
	return m
}

func problema(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("el cuerpo no es problem+json: %q", rec.Body.String())
	}
	if p.TraceID == "" {
		t.Error("el error salio sin traceId: no se puede encontrar en el log")
	}
	return p
}

func campoDelError(p httpx.Problem) string {
	if len(p.Errors) != 1 {
		return ""
	}
	return p.Errors[0].Field + ":" + p.Errors[0].Code
}

// Las imagenes de prueba se generan: no hay binarios en el repo que no se sepa
// de donde salieron.
func pngDe(t *testing.T, ancho, alto int, tono uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	img.Set(0, 0, color.RGBA{R: tono, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegDe(t *testing.T, ancho, alto int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, ancho, alto)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Un GIF de verdad, legible por DecodeConfig: rechazarlo tiene que ser por el
// tipo, no por estar roto.
func gifDe(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// La biblioteca estandar no escribe WebP. Es un WebP sin perdida de 1x1,
// comprobado con image.DecodeConfig.
func webp(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// rellenoHasta pega bytes al final de un PNG valido. DecodeConfig solo lee la
// cabecera, asi que la imagen sigue siendo legible: es la forma de tener un
// archivo valido de un tamano exacto.
func rellenoHasta(t *testing.T, base []byte, tamano int) []byte {
	t.Helper()
	if len(base) > tamano {
		t.Fatalf("la base ya ocupa %d", len(base))
	}
	return append(bytes.Clone(base), make([]byte, tamano-len(base))...)
}

func TestSubirUnPNGEs201ConLocationYLosDatosReales(t *testing.T) {
	b := nuevoBanco(t)
	img := pngDe(t, 3, 2, 10)

	rec := subir(t, b, quienSube(), archivo("logo.png", img))

	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	m := medio(t, rec)
	if loc := rec.Header().Get("Location"); loc != "/api/v1/media/"+m.Id.String() {
		t.Errorf("Location = %q", loc)
	}
	sum := sha256.Sum256(img)
	switch {
	case m.Mime != "image/png":
		t.Errorf("mime = %q", m.Mime)
	case m.Width != 3 || m.Height != 2:
		t.Errorf("dimensiones = %dx%d, se esperaba 3x2", m.Width, m.Height)
	case m.SizeBytes != int64(len(img)):
		t.Errorf("sizeBytes = %d, el archivo tiene %d", m.SizeBytes, len(img))
	case m.Sha256 != hex.EncodeToString(sum[:]):
		t.Errorf("sha256 = %q", m.Sha256)
	case m.Url != "/api/v1/public/media/"+m.Id.String():
		t.Errorf("url = %q", m.Url)
	case m.OriginalName == nil || *m.OriginalName != "logo.png":
		t.Errorf("originalName = %v", m.OriginalName)
	}

	// Los bytes guardados son los subidos, bajo la llave que sale del hash.
	rc, err := b.store.Open(context.Background(), llaveDe(sum[:]))
	if err != nil {
		t.Fatalf("no esta en el almacenamiento: %v", err)
	}
	defer rc.Close()
	guardado, _ := io.ReadAll(rc)
	if !bytes.Equal(guardado, img) {
		t.Error("lo guardado no es lo subido")
	}
}

func TestJPEGYWebPTambienSeAceptanConSusDimensiones(t *testing.T) {
	casos := []struct {
		nombre      string
		bytes       []byte
		mime        MedioMime
		ancho, alto int
	}{
		{"foto.jpg", jpegDe(t, 5, 4), "image/jpeg", 5, 4},
		{"icono.webp", webp(t), "image/webp", 1, 1},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := subir(t, nuevoBanco(t), quienSube(), archivo(c.nombre, c.bytes))
			if rec.Code != http.StatusCreated {
				t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
			}
			m := medio(t, rec)
			if m.Mime != c.mime || m.Width != c.ancho || m.Height != c.alto {
				t.Errorf("= %s %dx%d, se esperaba %s %dx%d", m.Mime, m.Width, m.Height, c.mime, c.ancho, c.alto)
			}
		})
	}
}

// El corazon de la historia: la misma imagen, con otro nombre, no crea otra
// fila ni otro archivo.
func TestSubirElMismoArchivoOtraVezEs200ConElMismoIdYNoDuplica(t *testing.T) {
	b := nuevoBanco(t)
	img := pngDe(t, 2, 2, 20)

	primera := medio(t, subir(t, b, quienSube(), archivo("logo.png", img)))
	rec := subir(t, b, quienSube(), archivo("otro-nombre.png", img))

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d, se esperaba 200: %s", rec.Code, rec.Body.String())
	}
	segunda := medio(t, rec)
	if segunda.Id != primera.Id {
		t.Errorf("id = %s, se esperaba el de la primera subida %s", segunda.Id, primera.Id)
	}
	if segunda.OriginalName == nil || *segunda.OriginalName != "logo.png" {
		t.Errorf("originalName = %v: la segunda subida no cambia el nombre", segunda.OriginalName)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("un 200 de dedup no lleva Location: no se creo nada")
	}
	if b.repo.inserts != 1 {
		t.Errorf("se intento insertar %d veces", b.repo.inserts)
	}
	if guardados, prov := b.archivos(t); guardados != 1 || prov != 0 {
		t.Errorf("hay %d archivos y %d provisionales; se esperaba 1 y 0", guardados, prov)
	}
}

func TestOtraImagenEsOtroRegistro(t *testing.T) {
	b := nuevoBanco(t)
	uno := medio(t, subir(t, b, quienSube(), archivo("a.png", pngDe(t, 2, 2, 1))))
	otro := subir(t, b, quienSube(), archivo("a.png", pngDe(t, 2, 2, 2)))

	if otro.Code != http.StatusCreated || medio(t, otro).Id == uno.Id {
		t.Errorf("estado = %d; dos imagenes distintas con el mismo nombre son dos registros", otro.Code)
	}
}

// El tipo sale de los bytes. El nombre y el Content-Type los escribe el
// cliente, y es como un .php termina llamandose .jpg.
func TestElTipoSaleDeLosBytesYNoDelNombreNiDelContentType(t *testing.T) {
	casos := map[string][]byte{
		"texto": []byte("no soy una imagen, aunque me llame logo.png"),
		"svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"gif":   gifDe(t),
		"html":  []byte("<!DOCTYPE html><html><script>alert(1)</script></html>"),
	}
	for nombre, contenido := range casos {
		t.Run(nombre, func(t *testing.T) {
			b := nuevoBanco(t)
			rec := subir(t, b, quienSube(), archivo("logo.png", contenido))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("estado = %d, se esperaba 400: %s", rec.Code, rec.Body.String())
			}
			if got := campoDelError(problema(t, rec)); got != "file:type" {
				t.Errorf("errors = %q, se esperaba file:type", got)
			}
			if guardados, prov := b.archivos(t); guardados+prov != 0 || b.repo.inserts != 0 {
				t.Errorf("quedo algo escrito: %d archivos, %d provisionales, %d filas", guardados, prov, b.repo.inserts)
			}
		})
	}
}

// Empieza como PNG y no lo es. DetectContentType solo mira la firma; lo que
// comprueba que la imagen se pueda leer es DecodeConfig.
func TestUnaFirmaDePNGConBasuraDetrasEs400(t *testing.T) {
	b := nuevoBanco(t)
	falso := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), 100)...)

	rec := subir(t, b, quienSube(), archivo("logo.png", falso))

	if rec.Code != http.StatusBadRequest || campoDelError(problema(t, rec)) != "file:type" {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if guardados, prov := b.archivos(t); guardados+prov != 0 {
		t.Errorf("quedaron %d archivos y %d provisionales", guardados, prov)
	}
}

func TestMasDe5MBEs413YNoQuedaNadaNiProvisional(t *testing.T) {
	b := nuevoBanco(t)
	grande := rellenoHasta(t, pngDe(t, 2, 2, 30), MaxBytes+1)

	rec := subir(t, b, quienSube(), archivo("grande.png", grande))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("estado = %d, se esperaba 413: %s", rec.Code, rec.Body.String())
	}
	if p := problema(t, rec); p.Code != httpx.CodePayloadTooLarge {
		t.Errorf("code = %q", p.Code)
	}
	if guardados, prov := b.archivos(t); guardados+prov != 0 || b.repo.inserts != 0 {
		t.Errorf("quedo algo escrito: %d archivos, %d provisionales, %d filas", guardados, prov, b.repo.inserts)
	}
}

// El borde: 5 MB justos pasan. Sin esta prueba, un `>=` en lugar de `>` en el
// tope rechazaria el archivo mas grande que el contrato promete aceptar.
func TestExactamente5MBPasa(t *testing.T) {
	b := nuevoBanco(t)
	justo := rellenoHasta(t, pngDe(t, 2, 2, 40), MaxBytes)

	rec := subir(t, b, quienSube(), archivo("justo.png", justo))

	if rec.Code != http.StatusCreated {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if m := medio(t, rec); m.SizeBytes != MaxBytes {
		t.Errorf("sizeBytes = %d", m.SizeBytes)
	}
}

// Un archivo mas chico que la cabecera que se lee para detectar el tipo. El
// WebP de prueba ocupa 34 bytes.
func TestUnArchivoMasChicoQueLaCabeceraSeLeeEntero(t *testing.T) {
	b := nuevoBanco(t)
	rec := subir(t, b, quienSube(), archivo("i.webp", webp(t)))
	if rec.Code != http.StatusCreated || medio(t, rec).SizeBytes != int64(len(webp(t))) {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnArchivoVacioEs400(t *testing.T) {
	rec := subir(t, nuevoBanco(t), quienSube(), archivo("vacio.png", nil))
	if rec.Code != http.StatusBadRequest || campoDelError(problema(t, rec)) != "file:required" {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSinElCampoFileEs400(t *testing.T) {
	rec := subir(t, nuevoBanco(t), quienSube())
	if rec.Code != http.StatusBadRequest || campoDelError(problema(t, rec)) != "file:required" {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
}

// Aceptar un campo que no se usa haria creer que se guardo algo que no.
func TestUnCampoDesconocidoAntesDelArchivoEs400YNoSeSube(t *testing.T) {
	b := nuevoBanco(t)
	rec := subir(t, b, quienSube(),
		campo{nombre: "alt", contenido: []byte("el logo")},
		archivo("logo.png", pngDe(t, 2, 2, 50)))

	if rec.Code != http.StatusBadRequest || campoDelError(problema(t, rec)) != "alt:unknown" {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	if b.repo.inserts != 0 {
		t.Error("se guardo el archivo a pesar del 400")
	}
}

func TestUnCuerpoQueNoEsMultipartEs400(t *testing.T) {
	b := nuevoBanco(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media", strings.NewReader(`{"file":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	b.servidor(quienSube()).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	problema(t, rec)
}

func TestSinSesionEs401(t *testing.T) {
	rec := subir(t, nuevoBanco(t), nil, archivo("logo.png", pngDe(t, 2, 2, 60)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado = %d", rec.Code)
	}
}

func TestSinPermisoDeSubirEs403YNoSeLeeElArchivo(t *testing.T) {
	b := nuevoBanco(t)
	rec := subir(t, b, con("media.read"), archivo("logo.png", pngDe(t, 2, 2, 70)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("estado = %d", rec.Code)
	}
	if guardados, prov := b.archivos(t); guardados+prov != 0 {
		t.Error("el guard dejo pasar la subida al almacenamiento")
	}
}

func TestLosDatosDeUnMedioPidenMediaRead(t *testing.T) {
	b := nuevoBanco(t)
	m := medio(t, subir(t, b, quienSube(), archivo("logo.png", pngDe(t, 2, 2, 80))))

	if rec := pedir(t, b, con("media.upload"), "/api/v1/media/"+m.Id.String()); rec.Code != http.StatusForbidden {
		t.Errorf("sin media.read: estado = %d", rec.Code)
	}
	rec := pedir(t, b, con("media.read"), "/api/v1/media/"+m.Id.String())
	if rec.Code != http.StatusOK || medio(t, rec).Id != m.Id {
		t.Errorf("con media.read: estado = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := pedir(t, b, con("media.read"), "/api/v1/media/"+uuid.NewString()); rec.Code != http.StatusNotFound {
		t.Errorf("id inexistente: estado = %d", rec.Code)
	}
}

// Lo que pide un <img> del visitante: sin sesion, con el tipo guardado y sin
// dejar que el navegador adivine otro.
func TestLosBytesPublicosSalenSinSesionConElTipoYCacheLarga(t *testing.T) {
	b := nuevoBanco(t)
	img := pngDe(t, 2, 2, 90)
	m := medio(t, subir(t, b, quienSube(), archivo("logo.png", img)))

	rec := pedir(t, b, nil, "/api/v1/public/media/"+m.Id.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	for cabecera, esperado := range map[string]string{
		"Content-Type":           "image/png",
		"Content-Length":         strconv.Itoa(len(img)),
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "public, max-age=31536000, immutable",
	} {
		if got := h.Get(cabecera); got != esperado {
			t.Errorf("%s = %q, se esperaba %q", cabecera, got, esperado)
		}
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("Content-Security-Policy = %q", h.Get("Content-Security-Policy"))
	}
	if !bytes.Equal(rec.Body.Bytes(), img) {
		t.Error("los bytes servidos no son los subidos")
	}
}

func TestUnMedioPublicoQueNoExisteEs404YUnIdQueNoEsUuid400(t *testing.T) {
	b := nuevoBanco(t)
	if rec := pedir(t, b, nil, "/api/v1/public/media/"+uuid.NewString()); rec.Code != http.StatusNotFound {
		t.Errorf("id inexistente: estado = %d", rec.Code)
	}
	rec := pedir(t, b, nil, "/api/v1/public/media/logo.png")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("id que no es uuid: estado = %d", rec.Code)
	}
	if p := problema(t, rec); len(p.Errors) != 1 || p.Errors[0].Field != "id" {
		t.Errorf("errors = %+v", p.Errors)
	}
}

// Una fila sin su archivo es un fallo del servidor, no un 404: alguien tiene que
// enterarse por el log.
func TestUnaFilaSinArchivoEs500(t *testing.T) {
	b := nuevoBanco(t)
	m := medio(t, subir(t, b, quienSube(), archivo("logo.png", pngDe(t, 2, 2, 100))))
	sum, _ := hex.DecodeString(m.Sha256)
	if err := os.Remove(filepath.Join(b.raiz, filepath.FromSlash(llaveDe(sum)))); err != nil {
		t.Fatal(err)
	}

	if rec := pedir(t, b, nil, "/api/v1/public/media/"+m.Id.String()); rec.Code != http.StatusInternalServerError {
		t.Errorf("estado = %d, se esperaba 500", rec.Code)
	}
}
