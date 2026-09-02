# 01 · Arquitectura

## La idea en una frase

Un **monolito modular** en Go donde la línea dura no está entre capas técnicas,
sino entre **lo que un fork no toca** y **lo que un fork reemplaza**.

```
platform/   identidad, sesión, permisos, HTTP, base, storage, paginación,
            auditoría, configuración, observabilidad
              ▲  nadie de aquí conoce a nadie de allá
              │
modules/    identity · content · media · settings · <tu dominio>
              ▲
app/        arranca, registra los módulos, monta el router
```

Si la separación fuera por capas globales (`handlers/`, `services/`,
`repositories/`), forkear significaría podar en cuatro carpetas a la vez. Con
esta forma, quitar el catálogo de una tienda es **borrar una carpeta y una
línea** en `app/modules.go`.

## Reglas de dependencia

Son cuatro y se verifican en pruebas (`09-calidad.md`), no se confían a la
disciplina:

1. `platform/` **no importa** `modules/` ni `app/`. Si la plataforma necesita
   saber algo de un módulo, ese algo va como interfaz que el módulo implementa
2. Un módulo **no importa** otro módulo. Si necesita datos ajenos, los pide por
   una interfaz declarada en su propio paquete y `app/` inyecta la
   implementación. Salvo `identity`, que es la excepción documentada: cualquiera
   puede depender de su tipo `Actor`
3. `app/` importa a todos. Es el único lugar que conoce el grafo completo
4. `handler → service → repo`, en un solo sentido. Un repo no llama a un
   service; un handler no habla SQL

La regla 2 es la que hace barato el fork, y la que más tienta romper. Cuando
duela, la salida correcta es un evento o una interfaz, no un import.

## Stack

| Pieza | Elección | Por qué |
|---|---|---|
| Lenguaje | Go 1.26 | — |
| HTTP | `net/http` estándar (`ServeMux` con patrones por método) | Desde 1.22 el ruteo del estándar alcanza. Un framework menos que envejezca |
| Base | Postgres 17 vía `pgx/v5` | — |
| Consultas | `sqlc` | SQL escrito a mano, tipos generados. Ni ORM ni `interface{}` |
| Migraciones | `goose`, embebidas por módulo | Ver `02-modulos.md` |
| Contrato | OpenAPI 3.1 + `oapi-codegen` (servidor) + `openapi-typescript` (cliente) | El contrato es la fuente, no la consecuencia |
| Validación | `go-playground/validator` en el borde | — |
| Sesión | JWT HS256 en cookie + refresh opaco rotado | Ver `06-flujos.md` |
| Logs | `log/slog` en JSON, con `traceId` por request | — |
| Pruebas | `testing` + `httptest` + `testcontainers-go` | Postgres real, no mock |
| Frontend | Nuxt 4 (Vue 3, TypeScript), Pinia, Reka UI headless | Ver `07-frontend.md` |
| Infra local | Docker Compose orquestado por devherd, con `edge` (Caddy) | Ver `08-infra-local.md` |

Producción **no está decidida**. `08-infra-local.md` documenta solo el local y
deja el hueco marcado en vez de inventar un despliegue que no se va a usar.

## Estructura de carpetas

```
go-starter/
├── api/                          # el backend en Go
│   ├── cmd/
│   │   ├── server/main.go        # el único binario que sirve HTTP
│   │   └── seed/main.go          # datos de desarrollo
│   ├── openapi.yaml              # el contrato. Fuente de verdad
│   ├── internal/
│   │   ├── app/
│   │   │   ├── app.go            # construye dependencias, arranca y apaga
│   │   │   ├── modules.go        # ← el registro. La lista de módulos vive aquí
│   │   │   └── router.go         # monta lo que cada módulo declaró
│   │   ├── platform/
│   │   │   ├── config/           # env → struct, sin defaults para secretos
│   │   │   ├── db/               # pool pgx, transacciones, runner de migraciones
│   │   │   ├── httpx/            # respuestas, problem+json, decode, middleware
│   │   │   ├── auth/             # emisión y verificación de sesión, CSRF
│   │   │   ├── rbac/             # permisos, políticas, guard
│   │   │   ├── paging/           # page, sort, filtros con lista blanca
│   │   │   ├── audit/            # created_by / updated_by automáticos
│   │   │   ├── storage/          # interfaz de archivos + implementación local
│   │   │   ├── validate/         # validador y traducción a FieldIssue
│   │   │   └── observ/           # slog, traceId, healthz, métricas
│   │   └── modules/
│   │       ├── identity/         # usuarios, roles, sesión
│   │       ├── content/          # páginas y bloques de la landing
│   │       ├── media/            # imágenes y archivos del contenido
│   │       └── settings/         # marca, navegación, pie, tema
│   ├── db/queries/               # .sql de sqlc, un archivo por módulo
│   └── test/                     # helpers compartidos de integración
├── web/                          # el frontend en Nuxt
│   ├── nuxt.config.ts            # routeRules: SSR en público, SPA en /admin
│   ├── app/
│   │   ├── pages/                # solo ruteo. La vista vive en el módulo
│   │   ├── modules/
│   │   │   ├── landing/          # components/ composables/ types.ts
│   │   │   ├── admin/            # el shell del dashboard
│   │   │   ├── content/          # editor de páginas y bloques
│   │   │   └── auth/             # login, sesión, guards
│   │   ├── shared/
│   │   │   ├── ui/               # primitivos propios sobre Reka UI
│   │   │   ├── api/              # cliente, interceptores, tipos generados
│   │   │   └── blocks/           # registro de bloques del CMS
│   │   └── assets/tokens/        # la piel del fork: color, tipografía, espacio
│   └── test/
├── docker/                       # Dockerfiles y config de Caddy
├── docs/                         # esto
├── compose.yaml
└── CLAUDE.md
```

## Qué no es un módulo

- **El dashboard no es un módulo del backend.** Es una vista que consume
  endpoints que ya existen. Si el dashboard necesita un endpoint que ningún
  módulo justifica por sí solo, la pregunta correcta es a qué módulo pertenece
  ese dato, no "creamos un módulo dashboard"
- **La landing tampoco.** La landing es `content` renderizado. El módulo que
  existe es el que administra el contenido; mostrarlo es trabajo del frontend
- **Un tipo de bloque no es un módulo.** Es un componente Vue más su esquema de
  props. Ver `03-modelo-de-datos.md`

## Relacionado

- Anatomía interna de un módulo: [`02-modulos.md`](02-modulos.md)
- Cómo se aprovecha esta forma al forkear: [`11-forks.md`](11-forks.md)
