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
- **`devherd exec` no existe.** No hay tal subcomando: todo lo que corre dentro
  de un contenedor va por `docker exec`
- **Con `docker exec`, no metas un `sh -lc`.** El shell de login rehace el
  `PATH` y deja fuera `/usr/local/go/bin`, así que el comando falla con
  `sh: go: not found` aunque Go esté instalado. Sin `sh`, `docker exec … go
  test` funciona

## Migraciones

```sh
# desarrollo: las aplica el servidor al arrancar (MIGRATE_ON_START=true en el compose)
devherd logs | grep "migracion aplicada"

# a mano, o como paso de despliegue. Aplica lo pendiente Y reconcilia el
# catálogo de permisos que declaran los módulos. Ver docs/03-modelo-de-datos.md
docker exec -w /workspace <contenedor-api> go run ./cmd/migrate
```

## El superadmin de desarrollo

```sh
./scripts/seed.sh    # = docker exec -w /workspace <contenedor-api> go run ./cmd/seed
```

`seed.sh` encuentra el contenedor del api de este checkout por las etiquetas de
compose, así que no hace falta saber el nombre que le puso devherd.

Migra, siembra los permisos y crea la cuenta con la que se entra a `/admin`:
`superadmin@go-starter.localhost` / `superadmin-de-desarrollo`, o lo que digan
`SEED_SUPERADMIN_EMAIL`, `SEED_SUPERADMIN_PASSWORD` y `SEED_SUPERADMIN_NAME`. Se
puede correr las veces que haga falta: las tres cosas son idempotentes, y volver
a sembrar reescribe la contraseña conservando el id.

Es `superadmin` y no `admin` a propósito: en desarrollo hace falta poder tocarlo
todo, incluidos los roles y los permisos.

**Solo corre con `APP_ENV=dev`**, y la cuenta que crea queda marcada en la fila
(`users.dev_seed`): esas credenciales no autentican fuera de desarrollo aunque
la base termine copiada a otro entorno. Ver `docs/03-modelo-de-datos.md`.

## Pruebas de integración

Las que ejecutan SQL de verdad se saltan solas sin `DATABASE_URL`, que fuera del
contenedor no está puesta. Van dentro:

```sh
docker exec -w /workspace <contenedor-api> \
    go test ./internal/modules/identity/ -run Integracion -v
```

Cada módulo lleva **su propia tabla de versiones**, `schema_migrations_<módulo>`.
Verlas es la forma rápida de saber qué módulos tienen esquema aplicado:

```sh
docker exec <contenedor-db> psql -U starter -d starter -c '\dt'
```

## Variables de entorno

Un solo `.env` en la raíz, **sin valores por omisión para secretos**. Una clave
de firma con default es una clave de firma en producción.

```
HOST_UID / HOST_GID      # para que lo generado dentro no quede de root
DB_NAME / DB_USER / DB_PASSWORD   # el compose arma DATABASE_URL con ellas
JWT_SIGNING_KEY          # sin default. Si falta, el proceso no arranca
COOKIE_SECURE            # false solo en local sin TLS
API_DOCS_ENABLED         # la UI de /api/v1/docs; se apaga sola fuera de dev
MIGRATE_ON_START         # el compose la pone en true para desarrollo
STORAGE_PATH             # sin default. Los archivos subidos; el compose la pone
NUXT_PUBLIC_API_BASE     # /api/v1        (lo usa el navegador)
NUXT_API_INTERNAL        # http://api:8080/api/v1  (lo usa el SSR)
```

`.env.example` es la referencia con valores de local.

**`STORAGE_PATH`** es donde `media` guarda los archivos subidos, y **no tiene
default**: en un contenedor, un directorio que no sea un volumen se pierde al
redesplegar, y eso se descubre cuando faltan las imágenes. El compose la pone en
`/workspace/.storage` —`api/.storage/` en el host, fuera de git y fuera de lo que
vigila air— y el CI en `.storage`, relativa a `api/`. Si no se puede escribir
ahí, el proceso no arranca.

Que el proceso **muera al arrancar** si falta un secreto, en vez de degradarse a
un default, es deliberado: un servicio a medio configurar que responde `200` es
peor que uno que no levanta.

### Las dos de la sesión

- **`JWT_SIGNING_KEY`** firma el `at` con HS256. Se genera con
  `openssl rand -base64 48`. Vacía no arranca, y se comprueba **dos veces**:
  `config.Load` la exige, y `app.Modules` se niega a armar el registro sin ella
  —así tampoco corren `cmd/migrate` ni `cmd/seed` con una llave vacía—.
  Cambiarla invalida todos los `at` emitidos: quien tenga sesión vuelve al
  login en la siguiente petición, no en quince minutos
- **`COOKIE_SECURE`** pone la bandera `Secure` en las cuatro cookies. Sin la
  variable, o con un valor que no se entiende (`si`, `yes`), queda en **true**:
  la falla segura es la que no manda la sesión por http. Sale de config y **no**
  de mirar `r.TLS`, porque detrás del edge todo llega por http y el flag se
  apagaría justo en producción

## Probar la API

Dos superficies, y la primera existe por una razón concreta:

- **`http://go-starter.localhost/api/v1/docs`** — UI generada del propio
  `openapi.yaml`. Vive en el **mismo origen** que la API, así que cuando llegue
  la sesión (fase 2) las cookies `HttpOnly` viajan solas y no hay que copiar
  tokens a mano. Una herramienta externa contra otro origen no puede hacer eso.
  Solo existe con `APP_ENV=dev` y `API_DOCS_ENABLED=true`; fuera de dev,
  `config.Load` la apaga aunque la variable quede puesta
- **`api/requests.http`** — las mismas peticiones para lanzarlas desde el editor

Con `curl`, una sesión necesita el CSRF: el primer `GET` siembra la cookie y
cada mutación la copia en `X-XSRF-TOKEN`. Sin eso, el login responde `403`.

```sh
B=http://go-starter.localhost/api/v1; J=$(mktemp)
curl -s -c $J $B/healthz >/dev/null                        # siembra XSRF-TOKEN
X=$(awk '$6=="XSRF-TOKEN"{print $7}' $J)
curl -s -b $J -c $J -X POST $B/auth/login -H "X-XSRF-TOKEN: $X" \
     -H 'Content-Type: application/json' \
     -d '{"email":"superadmin@go-starter.localhost","password":"superadmin-de-desarrollo"}'
curl -s -b $J $B/auth/me                                   # 200 con roles y permisos
```

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
