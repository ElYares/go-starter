# 05 · Contratos de API

## El contrato es la fuente

`api/openapi.yaml` se escribe **a mano y primero**. De ahí salen las dos mitades
generadas, y ninguna se edita:

```
api/openapi.yaml
   ├─ oapi-codegen        → un openapi_gen.go POR MÓDULO
   │                        api/internal/app/openapi_gen.go              (tag: salud)
   │                        api/internal/modules/settings/openapi_gen.go (tag: settings)
   └─ openapi-typescript  → web/app/shared/api/generated/schema.ts
```

**Uno por módulo, no uno global.** Sin filtrar, `oapi-codegen` emite una sola
`ServerInterface` con *todas* las operaciones del contrato, y entonces un único
struct tendría que implementar las de salud, las de `settings` y las de cada
módulo futuro. Eso obligaría a `app` a conocer cada operación, y es justo lo que
prohíbe la Decisión 002.

Lo que lo hace posible es `output-options.include-tags`, y por eso **cada
operación del contrato lleva el tag de su módulo**. Un tag mal puesto no falla:
simplemente deja la operación fuera de lo generado, y el módulo no la implementa.

Regenerar:

```sh
cd api && go generate ./...                    # el lado de Go
docker exec <web> sh -c 'npm run generate:api' # el lado de TypeScript
```

El contrato se monta en el contenedor de `web` como `/api/openapi.yaml`, para que
la ruta relativa `../api/openapi.yaml` valga igual dentro que fuera. En CI no hay
compose, y el script tiene que funcionar igual.

### Lo generado se monta a mano

`oapi-codegen` también genera el registro de rutas (`HandlerWithOptions`).
**No se usa.** Monta sobre un mux con middleware global para todas las
operaciones, y con eso se perdería el permiso declarado por ruta, `Route.Permission`
y la comprobación de arranque que se niega a levantar si una ruta exige un
permiso que ningún módulo declara.

En su lugar se montan los métodos de `ServerInterfaceWrapper` —que son
`http.HandlerFunc` normales: enlazan los parámetros del contrato y llaman a la
interfaz— sobre el router propio, ruta por ruta:

```go
w := &ServerInterfaceWrapper{Handler: m, ErrorHandlerFunc: errorDeParametro}
r.Get("", w.ListarSettings, rbac.Require("settings.read"))
```

`ErrorHandlerFunc` **no es opcional**: el de omisión responde con `http.Error`,
texto plano y sin `traceId`, y rompe la promesa de forma única justo en el error
más frecuente. Es el mismo agujero que el `404` de omisión de `net/http`.

Por qué en ese sentido y no al revés (generar el spec desde anotaciones del
código): un spec generado documenta lo que el código hace hoy, así que nunca
puede estar en desacuerdo con él — y por lo tanto **nunca avisa de un cambio
incompatible**. Escrito a mano, cambiar el contrato es un acto deliberado que
se ve en el diff.

Los tipos generados **se versionan en git**, y el CI los regenera y falla si el
diff no está vacío: el paso `Contrato al día` existe en los trabajos `api` y
`web`. Sin eso, un spec cambiado sin regenerar pasa en verde y el frontend
compila contra un contrato que ya no existe.

Lo que el CI **no** puede comprobar es que los patrones montados en `Routes()`
sean los del contrato, porque se escriben a mano. Eso lo cubre una prueba que le
pregunta al propio código generado qué rutas describe, con un mux espía. Ver
`internal/modules/settings/contrato_test.go`.

## Sesión: cookies, no cabeceras

Todo vive bajo un solo origen gracias al servicio `edge` (`08-infra-local.md`),
así que el navegador manda las cookies solo. **El frontend no guarda, no lee y
no adjunta tokens.**

| Cookie | Qué es | Vida | Path | Flags |
|---|---|---|---|---|
| `at` | JWT HS256 con `sub`, `roles`, `exp` | 15 min | `/api` | HttpOnly, Secure, SameSite=Lax |
| `rt` | Valor opaco de 256 bits | 14 días | `/api/v1/auth` | HttpOnly, Secure, SameSite=Lax |
| `XSRF-TOKEN` | Token CSRF | sesión | `/` | **Legible por JS**, Secure, SameSite=Lax |
| `has_session` | Pista `1`/ausente | 14 días | `/` | Legible por JS |

Dos que parecen de más y no lo son:

- **`XSRF-TOKEN` va en `Path=/`.** En `/api` la SPA no la ve, y entonces no
  puede mandar la cabecera. El síntoma es un `403` en el primer POST
- **`has_session` existe para no pedir refresh a quien nunca inició sesión.** Sin
  ella, cada carga anónima de la landing dispara un `POST /auth/refresh` que
  responde `401`: ruido en logs, latencia y un parpadeo de sesión en la UI

Mutación (`POST`/`PUT`/`PATCH`/`DELETE`) exige `X-XSRF-TOKEN` con el valor de la
cookie. Doble envío: quien no puede leer la cookie no puede forjar la cabecera.

## Endpoints de sesión

```
POST   /api/v1/auth/login     {email, password}  → 204 + cookies
POST   /api/v1/auth/refresh   (solo cookie rt)   → 204 + cookies rotadas
POST   /api/v1/auth/logout                        → 204, revoca y borra cookies
GET    /api/v1/auth/me                            → 200 {id, email, displayName, roles[], permissions[]}
```

- `login` responde `204` sin cuerpo. El perfil se pide con `me`, para que haya
  **un solo lugar** que defina qué sabe el frontend del usuario
- `me` devuelve `permissions[]` resueltos, no solo roles: el frontend oculta lo
  que no aplica sin reimplementar el modelo de permisos. Ocultar es
  **conveniencia, no seguridad** — la autorización real vive en el servidor

## Superficie pública

```
GET /api/v1/public/pages/{slug}   → la versión PUBLICADA, o 404
GET /api/v1/public/settings       → solo las claves marcadas como públicas
GET /api/v1/public/media/{id}     → redirección o bytes
```

Separada por prefijo a propósito. Un endpoint público es una decisión, no un
descuido: `settings` tiene claves privadas (llaves de terceros, correos
internos) y el filtro de qué es público vive en servidor, no en el frontend.

## Versionado

`/api/v1` desde el primer día. Cambiar el contrato de forma incompatible
—quitar un campo, cambiar un tipo, volver obligatorio lo opcional— abre `/v2`;
agregar un campo opcional no. Un starter cuyos forks salen a producción no
puede romper clientes que no controla.

## Reglas de colección y de error

Viven en [`04-reglas-de-crud.md`](04-reglas-de-crud.md), secciones 3 y 6. No se
repiten aquí para que haya una sola fuente.

## Trampas conocidas

- **Una ruta inexistente también sale como problem+json.** El `404` de omisión de
  `net/http` es texto plano y rompe la promesa de forma única sin que nadie lo
  note, porque un 404 «se ve bien» en el navegador. Lo cubre un patrón comodín
  en el router
- **El middleware de sesión contesta antes de resolver la ruta.** Una ruta
  inexistente da `401` sin cookies y `404` con ellas. Una prueba de `404` tiene
  que autenticar primero, o afirma un `401` creyendo que afirma un `404`
- **Los mensajes de validación se traducen en el servidor** (`platform/validate`),
  porque la librería contesta en inglés y nombrando el campo de Go (`DisplayName`)
  en vez del del contrato (`displayName`) — con eso un cliente resalta el campo
  equivocado en el formulario. El `code` sigue siendo lo estable: si un fork
  quiere traducir por su cuenta, tiene con qué. Lo que no se hace es traducir en
  los dos lados
- **El token CSRF rota en cada refresh autenticado.** El cliente debe leerlo de
  la cookie en cada petición, no cachearlo al arrancar
