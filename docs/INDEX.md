# Índice de documentación — go-starter

Una línea por documento para poder elegir qué leer sin abrir todo.
Léelo antes que cualquier otro archivo.

| Archivo | Qué contiene | Cuándo leerlo |
|---|---|---|
| [`01-arquitectura.md`](01-arquitectura.md) | Plataforma vs. módulos, reglas de dependencia, stack y estructura completa de carpetas | Antes de escribir la primera línea |
| [`02-modulos.md`](02-modulos.md) | Anatomía de un módulo, `module.go`, registro, migraciones y permisos propios | Antes de crear o borrar un módulo |
| [`03-modelo-de-datos.md`](03-modelo-de-datos.md) | Convenciones de tablas, auditoría, identidad, contenido versionado, medios y settings | Antes de la primera migración |
| [`04-reglas-de-crud.md`](04-reglas-de-crud.md) | **El molde.** Rutas, códigos, paginación, filtros, concurrencia, permisos y checklist de "hecho" | Antes de cada endpoint nuevo |
| [`05-contratos-api.md`](05-contratos-api.md) | OpenAPI como fuente, errores RFC 7807, colecciones, sesión por cookie, CSRF, versionado | Al conectar back y front |
| [`06-flujos.md`](06-flujos.md) | Los flujos completos: sesión, SSR de la landing, publicación de contenido, subida de medios | Antes de tocar auth o contenido |
| [`07-frontend.md`](07-frontend.md) | Estructura Nuxt, componentes por módulo, tokens de diseño, capa de API, guards, los cuatro estados | Antes de la primera vista |
| [`08-infra-local.md`](08-infra-local.md) | Compose, servicio `edge`, devherd, HMR detrás del proxy. Producción: sin decidir | Fase 0, y cada vez que algo no levanta |
| [`09-calidad.md`](09-calidad.md) | Pruebas, testcontainers, límites verificados, CI y observabilidad | Antes de la fase 1 |
| [`10-roadmap.md`](10-roadmap.md) | Las fases en orden, con criterio de "hecho" verificable | Al empezar cualquier sesión |
| [`11-forks.md`](11-forks.md) | Cómo se forkea: qué se toca, qué no, y la receta para agregar o quitar un módulo | El día que nace un proyecto derivado |
| `12-infra-prod.md` | **No existe todavía, a propósito.** El destino de producción no está decidido; `08` fija las restricciones que ya condicionan el código | Cuando el primer fork salga a producción |

## Dónde vive el pensamiento

Este directorio documenta **qué se construye y cómo**. El **por qué** de cada
decisión vive en el vault, no aquí:

- Estado de hoy: `10 Projects/go-starter/go-starter - Estado`
- Decisiones: `10 Projects/go-starter/Decisions/`
- Backlog (HU/CU): `10 Projects/go-starter/Backlog/`

Si un documento de aquí contradice una decisión del vault, gana el vault y este
archivo está desactualizado.
