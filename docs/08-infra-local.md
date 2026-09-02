# 08 · Infraestructura local

## Qué levanta

```
                    ┌─────────────┐
   navegador ──────►│ edge (Caddy)│  go-starter.localhost
                    └──────┬──────┘
              /api/*       │        todo lo demás
          ┌───────────────┴───────────────┐
          ▼                               ▼
    ┌──────────┐                    ┌──────────┐
    │   api    │  Go, :8080         │   web    │  Nuxt, :3000
    └────┬─────┘                    └────┬─────┘
         │                               │
         ▼                               └──► api:8080 (SSR, red interna)
    ┌──────────┐
    │    db    │  Postgres 17
    └──────────┘
```

Servicios: `edge`, `api`, `web`, `db`. **Ningún puerto publicado al host.** Todo
entra por el proxy de devherd.

## El servicio `edge` no es un lujo

Es lo que sostiene el modelo de sesión entero. Con un solo origen, las cookies
`HttpOnly` funcionan sin CORS, sin `SameSite=None` y sin que el frontend toque
un token. Separar los orígenes —`localhost:3000` para el front y `:8080` para la
API— tira la decisión completa de sesión y obliga a inventar otra.

Reglas del `edge`, en orden:

```
/api/*   → api:8080
/*       → web:3000      (incluye el websocket de HMR de Nuxt)
```

## devherd

```sh
devherd up              # levanta el stack
devherd proxy apply     # OBLIGATORIO después de cualquier down
devherd logs            # seguir logs
devherd down            # apagar
```

**Un proyecto nuevo hay que parquearlo antes**, o `proxy apply` lo ignora **sin
decirlo**: el comando termina en `proxy status: applied`, lista los dominios que
sí aplicó, y el tuyo no aparece ni entre los aplicados ni entre los saltados —
simplemente no existe para devherd. El síntoma es el `200` con cuerpo vacío, que
parece un problema del stack cuando en realidad el stack está perfecto.

```sh
devherd park /home/elyares/develop/labs/go-starter   # el proyecto, NO el padre
devherd list | grep go-starter                       # debe salir con su dominio
devherd proxy apply                                  # y ahora sí lo incluye
```

Parquear el **directorio padre** en vez del proyecto registra de paso todo lo
que parezca proyecto ahí dentro, incluidos repos sin compose.

**`devherd proxy apply` pide sudo** para escribir los dominios en `/etc/hosts`.
Desde una sesión no interactiva falla con `sudo: a terminal is required`, y el
stack queda arriba pero sin dominio — con el mismo síntoma del `200` vacío de
abajo. Hay que correrlo desde una terminal donde se pueda teclear la contraseña.

**Tras un `down`, sin `proxy apply` el dominio responde `200` con cuerpo
vacío.** No es un `502` ni un error: es un `200` mentiroso. Se detecta mirando
el tamaño de la respuesta, no el código:

```sh
curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://go-starter.localhost/
```

Otros puntos que ya dolieron en el proyecto hermano y aplican igual aquí:

- **Arranque frío de verdad** es borrar el volumen, no reiniciar:
  `docker volume rm devherd-go-starter-<hash>_db_data`
- **Todo comando de Node va por `docker exec`**, no desde el host: el
  `node_modules` del host está vacío a propósito, porque es un volumen nombrado
- **Cambiar `go.mod` obliga a reiniciar el contenedor `api`**, igual que cambiar
  dependencias en cualquier stack con recarga en caliente
- **La base desde un gestor gráfico:** la IP sale de
  `docker inspect devherd-go-starter-<hash>-db-1`. Cambia en cada `down`+`up`

## Variables de entorno

Un solo `.env` en la raíz, **sin valores por omisión para secretos**. Una clave
de firma con default es una clave de firma en producción.

```
POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_DB
DATABASE_URL
JWT_SIGNING_KEY          # sin default. Si falta, el proceso no arranca
COOKIE_SECURE            # false solo en local sin TLS
STORAGE_PATH
NUXT_PUBLIC_API_BASE     # /api/v1        (lo usa el navegador)
NUXT_API_INTERNAL        # http://api:8080/api/v1  (lo usa el SSR)
```

Que el proceso **muera al arrancar** si falta un secreto, en vez de degradarse a
un default, es deliberado: un servicio a medio configurar que responde `200` es
peor que uno que no levanta.

## Probar la API

Dos superficies, y la primera existe por una razón concreta:

- **`http://go-starter.localhost/api/v1/docs`** — UI generada del propio
  `openapi.yaml`. Vive en el **mismo origen** que la API, así que cuando llegue
  la sesión (fase 2) las cookies `HttpOnly` viajan solas y no hay que copiar
  tokens a mano. Una herramienta externa contra otro origen no puede hacer eso.
  Solo existe con `APP_ENV=dev` y `API_DOCS_ENABLED=true`; fuera de dev,
  `config.Load` la apaga aunque la variable quede puesta
- **`api/requests.http`** — las mismas peticiones para lanzarlas desde el editor

## Recarga en caliente

- **Go:** `air` dentro del contenedor. Recompila y reinicia en cada guardado.
  Vigila también `.yaml`, así que tocar `openapi.yaml` recarga el proceso y la
  UI de la API sirve el contrato nuevo
- **Nuxt:** `nuxt dev` detrás del `edge`. El HMR usa websocket, así que el proxy
  tiene que pasar el `Upgrade`. Si no, el síntoma es "los cambios no se ven
  hasta recargar a mano" — no un error, solo silencio

### El HMR detrás del proxy

El navegador habla con el `edge` en el puerto 80, no con Vite en el 3000 —que no
está publicado—. Sin decírselo, el cliente de HMR abre el websocket contra el
3000 y queda muerto en silencio:

```ts
// web/nuxt.config.ts
vite: {
  server: {
    ws: { clientPort: 80 },        // ← el navegador habla con el proxy
    allowedHosts: ['go-starter.localhost'],
  },
}
```

Dos detalles que cuestan tiempo:

- **Va en `ws`, no en `hmr`.** Vite 8 renombró `server.hmr.*` a `server.ws.*`
- **El aviso de deprecación de `server.hmr` sale igual**, porque lo pone Nuxt
  internamente. No es tu configuración: comprobarlo persiguiendo el aviso es una
  hora perdida
- **No se fija `ws.host`.** Sin él, el cliente usa el hostname de la página, que
  es lo correcto y sobrevive a un cambio de dominio

Comprobarlo sin abrir el navegador:

```sh
curl -s -i -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
     -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
     http://go-starter.localhost/_nuxt/ | head -1
# HTTP/1.1 101 Switching Protocols
```

## Producción: sin decidir

**Está deliberadamente en blanco.** devherd es una herramienta local y no se usa
en producción; el destino real (VPS con compose, Kubernetes, PaaS) todavía no
está elegido, y documentar un despliegue que no se va a usar es peor que no
tener el documento.

Lo que sí queda fijado desde ahora, porque condiciona el código:

- El binario de Go compila **estático**, sin cgo, para que la imagen final sea
  `scratch` o `distroless`
- La configuración entra **solo por entorno**. Ningún archivo de configuración
  por ambiente dentro de la imagen
- Las migraciones corren como **paso explícito de despliegue**, no al arrancar
  el servidor: dos réplicas migrando a la vez es una carrera
- `GET /healthz` (¿el proceso vive?) y `GET /readyz` (¿la base responde?) son
  endpoints distintos y los dos existen desde el primer día

Cuando se decida el destino, esto se reemplaza por `12-infra-prod.md` y se
escribe una decisión en el vault.
