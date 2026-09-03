# 02 · Anatomía de un módulo

Un módulo es un **componente autocontenido**: trae sus rutas, sus migraciones,
sus permisos y sus dependencias declaradas. Nadie fuera de él sabe qué tiene
dentro; `app/` solo lo monta.

La prueba de que un módulo está bien hecho: **borrar su carpeta y su línea del
registro deja el proyecto compilando**.

## Archivos

```
internal/modules/content/
├── module.go        # el contrato con app/: quién soy y qué monto
├── domain.go        # tipos del dominio y sus errores. Sin SQL, sin HTTP
├── service.go       # las reglas de negocio. Sin SQL, sin HTTP
├── repo.go          # persistencia. Sin reglas de negocio
├── handler.go       # HTTP: decodifica, llama al service, responde
├── ports.go         # interfaces de lo que este módulo necesita de fuera
├── migrations/      # 0001_pages.sql, 0002_page_versions.sql …
└── *_test.go
```

Cuando un módulo pasa de ~600 líneas por archivo, se parte por recurso
(`handler_pages.go`, `service_pages.go`), nunca por capa nueva.

## El contrato: `module.go`

```go
package content

// Module es lo único que app/ conoce de este paquete.
type Module struct {
    svc *Service
}

func New(deps Deps) *Module { … }

// Name identifica al módulo en logs, métricas y tabla de migraciones.
func (m *Module) Name() string { return "content" }

// Permissions declara los permisos que este módulo inventa.
// rbac los siembra al arrancar; nadie los escribe a mano en la base.
func (m *Module) Permissions() []rbac.Permission {
    return []rbac.Permission{
        {Key: "content.page.read",    Desc: "Ver páginas"},
        {Key: "content.page.write",   Desc: "Crear y editar páginas"},
        {Key: "content.page.publish", Desc: "Publicar una versión"},
    }
}

// Migrations son las suyas, embebidas. Ver "Migraciones" abajo.
//go:embed migrations/*.sql
var migrationsFS embed.FS

// El fs.Sub no es adorno: goose espera los .sql en la RAÍZ del sistema de
// archivos que recibe, y el embed los deja bajo `migrations/`. Sin recortar el
// prefijo no encuentra ninguna y no aplica nada — sin error.
func (m *Module) Migrations() fs.FS {
    sub, err := fs.Sub(migrationsFS, "migrations")
    if err != nil {
        panic("content: migrations/ no esta embebido: " + err.Error())
    }
    return sub
}

// Routes es el único lugar donde este módulo aparece en el router.
func (m *Module) Routes(r *httpx.Router) {
    r.Group("/api/v1", func(r *httpx.Router) {
        r.Get   ("/pages",              m.list,    rbac.Require("content.page.read"))
        r.Post  ("/pages",              m.create,  rbac.Require("content.page.write"))
        r.Get   ("/pages/{id}",         m.get,     rbac.Require("content.page.read"))
        r.Put   ("/pages/{id}",         m.update,  rbac.Require("content.page.write"))
        r.Delete("/pages/{id}",         m.delete,  rbac.Require("content.page.write"))
        r.Post  ("/pages/{id}/publish", m.publish, rbac.Require("content.page.publish"))
    })
    // Lo público va aparte y sin guard, a propósito y a la vista.
    r.Get("/api/v1/public/pages/{slug}", m.getPublic)
}
```

`Name`, `Permissions`, `Migrations` y `Routes` son la interfaz `app.Module`.
Un módulo que no la cumple no compila; uno que la cumple se monta solo.

## El registro: `app/modules.go`

```go
func modules(deps Deps) []Module {
    return []Module{
        identity.New(deps.Identity),   // primero: los demás dependen de users
        settings.New(deps.Settings),
        media.New(deps.Media),
        content.New(deps.Content),     // usa media por interfaz, no por import
        // catalog.New(deps.Catalog),  // ← un fork agrega su dominio aquí
    }
}
```

El orden es significativo: **es el orden en que corren las migraciones**. Un
módulo solo puede tener llave foránea hacia otro que se registre antes que él,
y en la práctica eso significa hacia `identity`. Si dos módulos se necesitan
mutuamente, uno de los dos está mal cortado.

## Migraciones

Viven dentro del módulo, no en un directorio global, porque borrar el módulo
tiene que borrar también su esquema del repositorio.

- Se embeben con `//go:embed migrations/*.sql`
- Numeración **local al módulo**: `0001_`, `0002_`. No hay contador global
- Cada módulo lleva **su propia tabla de versiones**:
  `goose.SetTableName("schema_migrations_content")`. Con una tabla compartida,
  el orden de dos módulos independientes se vuelve global y frágil
- El runner de `platform/db` recorre los módulos en el orden del registro y
  aplica lo pendiente de cada uno
- **Migrar es un paso explícito**, no un efecto secundario de arrancar: en
  producción va `cmd/migrate`, porque dos réplicas migrando a la vez es una
  carrera. En desarrollo `MIGRATE_ON_START=true` lo hace el propio servidor, y el
  compose ya lo trae puesto
- **`platform/db` no conoce el tipo `Module`.** Declara su propia interfaz
  `Migratable` (nombre + migraciones) del lado del consumidor: importar `app`
  sería un import hacia arriba y rompería la regla 1

**La trampa:** una migración que transforma datos pasa siempre si corre contra
una base vacía — no hay filas que transformar. Para probarla de verdad hay que
migrar hasta `N-1`, sembrar, y entonces aplicar. Ver `09-calidad.md`.

## Dependencias entre módulos: `ports.go`

Un módulo nunca importa otro módulo. Declara lo que necesita:

```go
// content/ports.go
type MediaStore interface {
    URLFor(ctx context.Context, id uuid.UUID) (string, error)
    Exists(ctx context.Context, id uuid.UUID) (bool, error)
}
```

`app/` conecta `media.Module` a esa interfaz. Consecuencias que sí importan:

- `content` se prueba con un `MediaStore` de mentira, sin levantar `media`
- Un fork que no tenga imágenes implementa la interfaz con dos líneas
- El día que `media` se vaya a S3 o a otro servicio, `content` no se entera

Cuando la relación es "avísame cuando pase algo" y no "dame este dato", la
respuesta correcta es un evento en `platform`, no una interfaz más.

## Checklist de un módulo nuevo

- [ ] `module.go` implementa las cuatro funciones y no exporta nada más
- [ ] Ningún import de otro `modules/…` (`09-calidad.md` lo verifica)
- [ ] Sus permisos están declarados, no escritos a mano en una migración
- [ ] Sus migraciones son locales y con su propia tabla de versiones
- [ ] Sus endpoints cumplen el molde de [`04-reglas-de-crud.md`](04-reglas-de-crud.md)
- [ ] Está en `openapi.yaml` y los tipos del frontend están regenerados
- [ ] Borrar la carpeta y su línea del registro deja el proyecto compilando
