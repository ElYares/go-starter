# 05 · Contratos de API

## El contrato es la fuente

`api/openapi.yaml` se escribe **a mano y primero**. De ahí salen dos cosas
generadas, y ninguna se edita:

```
api/openapi.yaml
   ├─ oapi-codegen        → api/internal/app/openapi_gen.go   (interfaces del servidor)
   └─ openapi-typescript  → web/app/shared/api/generated/      (tipos del cliente)
```

Por qué en ese sentido y no al revés (generar el spec desde anotaciones del
código): un spec generado documenta lo que el código hace hoy, así que nunca
puede estar en desacuerdo con él — y por lo tanto **nunca avisa de un cambio
incompatible**. Escrito a mano, cambiar el contrato es un acto deliberado que
se ve en el diff.

Los tipos generados **se versionan en git**. Nadie comprueba automáticamente que
estén al día: si el spec cambió y el `generated/` no, el frontend compila contra
un contrato que ya no existe. Regenerar es parte de la definición de "hecho".

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

- **El middleware de sesión contesta antes de resolver la ruta.** Una ruta
  inexistente da `401` sin cookies y `404` con ellas. Una prueba de `404` tiene
  que autenticar primero, o afirma un `401` creyendo que afirma un `404`
- **Los mensajes de validación salen en inglés** si se dejan los de la librería.
  El `code` de cada `FieldIssue` es lo estable; traducir en el cliente o
  registrar un traductor en el servidor, pero no las dos cosas
- **El token CSRF rota en cada refresh autenticado.** El cliente debe leerlo de
  la cookie en cada petición, no cachearlo al arrancar
