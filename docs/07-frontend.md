# 07 · Frontend

## Un repo, dos modos de render

```ts
// nuxt.config.ts
routeRules: {
  '/**':       { ssr: true  },   // landing: SEO real, meta tags, previews
  '/admin/**': { ssr: false },   // dashboard: nadie indexa un panel privado
}
```

La landing se renderiza en servidor porque un fork tipo tienda o landing vive de
que Google y los previews de WhatsApp vean contenido. El dashboard es SPA porque
renderizarlo en servidor obligaría a reenviar cookies de sesión en cada
navegación a cambio de cero beneficio.

## Estructura

```
web/app/
├── pages/                    # SOLO ruteo. Cada archivo importa una vista
│   ├── index.vue             → modules/landing
│   ├── [...slug].vue         → modules/landing (páginas del CMS)
│   └── admin/…               → modules/admin
├── modules/
│   ├── landing/
│   │   ├── components/       # componentes de esta pantalla, no globales
│   │   ├── composables/
│   │   └── views/
│   ├── admin/                # shell: navegación lateral, encabezado, layout
│   ├── content/              # editor de páginas y bloques
│   ├── auth/                 # login, store de sesión, guards
│   └── settings/             # la pantalla de configuración del sitio
├── shared/
│   ├── ui/                   # los primitivos propios. Ver abajo
│   ├── blocks/               # registro de tipos de bloque del CMS
│   └── api/
│       ├── client.ts         # instancia, interceptores
│       ├── generated/        # tipos desde openapi.yaml. NO se edita
│       └── errors.ts         # ApiError
└── assets/tokens/            # la piel: color, tipografía, espacio, radio
```

**`pages/` no tiene lógica.** Es la tabla de ruteo de Nuxt y nada más. La vista
real vive en su módulo, lo que permite mover una pantalla entre rutas sin
tocarla, y borrar un módulo entero de un fork sin dejar restos.

Un componente vive en `shared/ui/` **cuando lo usan dos módulos**. Antes de eso
vive en el módulo que lo usa. Lo contrario —todo a `shared/` por si acaso—
produce una carpeta de doscientos componentes donde nadie sabe cuál está vivo.

## Componentes propios sobre primitivas headless

La regla es: **el comportamiento se hereda, la piel se escribe**.

- Reka UI aporta lo que es caro y peligroso hacer a mano: foco atrapado en un
  diálogo, navegación por teclado de un menú, roles ARIA, `combobox`, `popover`
- Encima va un componente propio en `shared/ui/` con los tokens del proyecto

```
shared/ui/
  BaseButton.vue     BaseInput.vue     BaseSelect.vue
  BaseDialog.vue     BaseTable.vue     BaseToast.vue
  BaseField.vue      BaseBadge.vue     BaseEmptyState.vue
  index.ts           ← el único punto de entrada; las vistas importan de aquí
```

Ninguna vista importa Reka UI directamente. Si lo hiciera, cambiar de librería
—o quitarla— sería tocar cincuenta archivos en vez de diez. **Es una prueba,
no una convención:** `test/limites-ui.spec.ts` lee los `.vue` y `.ts` de `app/`
y falla si alguno fuera de `shared/ui/` menciona `reka-ui`. Es el gemelo de
`api/internal/app/limites_test.go`, y como aquél, falla también si no leyó
ningún archivo: un guardia que pasa por vacuidad no guarda nada.

Los `Base*` no usan nada de Nuxt a propósito —importan `ref`, `computed` y
`useId` desde `vue` de forma explícita—. Eso es lo que permite probarlos con
`@vue/test-utils` sin levantar el arnés de Nuxt, y lo que deja sacarlos de aquí
sin arrastrar el framework.

### Tokens: lo que un fork cambia primero

```css
/* assets/tokens/base.css */
:root {
  --color-bg: …;         --color-surface: …;   --color-surface-hover: …;
  --color-text: …;       --color-text-muted: …;
  --color-accent: …;     --color-accent-strong: …;  --color-on-accent: …;
  --color-brand: …;      --color-brand-deep: …;     --color-on-brand: …;   --color-on-brand-muted: …;  /* el panel del login */
  --color-border: …;     --color-ok: …;        --color-danger: …;
  --color-scrim: …;      /* el velo detrás de un diálogo */
  --font-sans: …;        --font-mono: …;
  --text-sm: …;          --text-base: …;   --text-lg: …;   --text-display: …;
  --space-1: … --space-8: …;
  --radius-sm: …;        --radius-md: …;   --radius-lg: …;  --radius-full: …;
  --focus-ring: …;
}
```

Dos reglas que ya costaron caro en el proyecto hermano:

- **Un color de marca no sirve automáticamente para texto.** Un verde de marca
  puede dar 2.5:1 de contraste. Por eso existe `--color-accent-strong` aparte:
  el de marca pinta rellenos, el fuerte pinta texto y bordes
- **Todo token de relleno necesita su par de texto.** `--color-on-accent` existe
  como token, y no como un `#fff` dentro de un botón, porque en tema oscuro el
  par se invierte

Las dos están probadas, y por una razón concreta: `--color-accent` sobre
`--color-on-accent` en tema claro da **4.53:1**. Tres centésimas de margen. El
primer fork que meta su color de marca lo rompe, y no hay ningún síntoma visible.

- `test/tokens-contraste.spec.ts` lee el CSS de verdad —no una copia— y mide los
  21 pares en los dos temas contra 4.5:1. Un fondo nuevo sin su par de texto no
  es un olvido: es un par que falta en esa lista
- `test/tokens-sin-literales.spec.ts` falla si un `.vue` de `app/` escribe un
  color, un espacio o un radio a mano. El grosor de un borde (`1px`) sí se
  permite: no es un token en ningún sistema, e inventar `--border-1` sería
  ceremonia que nadie cambia

## Bloques del CMS

Un tipo de bloque son dos cosas que viajan juntas: el JSON Schema que valida sus
`props` en el api (`api/internal/modules/content/bloques/<tipo>.json`) y su
componente aquí.

```ts
// shared/blocks/registry.ts
export const bloques = {
  hero:     { component: HeroBlock,     label: 'Portada' },
  features: { component: FeaturesBlock, label: 'Caracteristicas' },
  texto:    { component: TextoBlock,    label: 'Texto' },
}
```

- **`RenderDeBloques`** recorre `page.blocks` y resuelve `type` contra el
  registro. Lo usa la landing y lo usará la vista previa del editor
- Un `type` desconocido **no revienta**: cae a `BloqueDesconocido`, que no pinta
  nada y deja el aviso en el log (en SSR, el del servidor de Nuxt). Un fork que
  borra un bloque no puede tumbar páginas ya publicadas
- **`registry.spec.ts` compara el registro con los archivos del catálogo del
  api** y falla si uno tiene un tipo que el otro no. Para verlos, el contenedor
  de web monta `bloques/` de solo lectura (`compose.yaml`)
- `resolverBloque` usa `Object.hasOwn`: un `type: "constructor"` encontraría
  algo en el prototipo y lo intentaría pintar
- Los bloques **no importan nada de Nuxt**, como los `Base*`: el enlace del hero
  es un `<a>`, que funciona sin JavaScript, y se prueban sin el arnés
- **El texto nunca va por `v-html`.** El esquema dice texto plano, y quien tiene
  `content.page.write` no puede inyectar marcado en la landing pública
- Importaciones directas, no `() => import()`: son tres componentes chicos que la
  portada usa siempre
- **El editor usa el mismo registro y el mismo catálogo.** `shared/blocks/catalogo.ts`
  importa los JSON Schema del api en el build (`import.meta.glob`), y de ahí el
  editor arma los formularios. Cambiar el lenguaje visual de un fork es escribir
  un esquema y un componente por tipo; el editor no se toca
- Cada propiedad del esquema lleva `title`: es la etiqueta del campo en el
  editor. `catalogo.spec.ts` falla si falta

## El editor de páginas

`/admin/paginas` lista las páginas y `/admin/paginas/{id}` las edita (CU-004). Todo
vive en `modules/content/`: la lógica sin Vue en `editor.ts`, las llamadas en
`api.ts`, y componentes que reciben las operaciones como funciones, como
`ListaDeSettings`.

**Los formularios salen del JSON Schema del catálogo** (`CampoDeEsquema.vue`,
recursivo). Vive en `shared/formularios/` porque lo usan dos módulos —el editor
de páginas y la configuración del sitio—, junto con sus funciones puras
(`esquema.ts`: `valorInicial`, `mover`, `erroresPorCampo`). Entiende lo que usan
los esquemas del starter:

| En el esquema | En el editor |
|---|---|
| `string` | entrada; área de texto si `maxLength ≥ 300` y no tiene `pattern` |
| `object` requerido | sus campos |
| `object` opcional | "Agregar …" / "Quitar …" |
| `array` | un grupo por elemento, con subir, bajar y quitar; respeta `minItems` y `maxItems` |
| `string` con `format: media-id` | un campo de imagen: vista previa, subir y quitar (`CampoDeMedio.vue`) |
| `$ref` a `#/$defs/…` | lo que nombra; se resuelve al cargar (`resolverReferencias`) |
| otra cosa | su JSON, sin editar, y se conserva al guardar |

Un opcional vaciado se **quita** del objeto: "sin bajada" no es `subtitle: ""`.
Un bloque nuevo nace con lo obligatorio de su esquema y nada más.

Reglas del editor que no se ven en el marcado:

- **Cada campo se nombra con la ruta del api** (`blocks[2].props.items[0].title`),
  así el `errors[]` de un 400 llega solo al campo que lo tiene, y el bloque se
  marca en rojo. Lo escrito no se toca
- **Un 409 no guarda encima.** Avisa cuándo se guardó la otra versión —la lee,
  no la aplica— y ofrece descartar y recargar. No dice *quién*: el api da
  `updatedBy` como uuid y no hay con qué resolver el nombre
- **No se publica con cambios sin guardar**: publicar apunta a una versión
  guardada, y con cambios en pantalla publicaría algo distinto de lo que se ve
- **Un tipo de bloque desconocido** se muestra como no reconocido, con su JSON,
  y se deja quitar. Viaja tal cual al guardar; el servidor lo rechaza, y un
  aviso lo dice antes
- Revertir es "Publicar esta versión" en el historial. No hay borrar versiones
- Salir con cambios sin guardar pregunta, dentro del dashboard y al cerrar la
  pestaña (`EditorView.vue`)
- **La copia del borrador es por JSON, no `structuredClone`**: la página llega
  en un proxy reactivo y `structuredClone` lanza `DataCloneError` con él. El
  editor se quedaba en "cargando" sin un error visible

## La configuración del sitio

`/admin/configuracion` lista las claves y `/admin/configuracion/{clave}` edita una
(CU-006). El formulario sale del esquema de la clave
(`api/internal/modules/settings/esquemas/`), importado en el build como el
catálogo de bloques (`modules/settings/esquemas.ts`), y el compose monta esa
carpeta en web. El 400 por campo, el 409 y el aviso de salir con cambios son los
del editor de páginas.

- **El guardado no manda `isPublic`**: ausente conserva la visibilidad
  (Decision 025). Esta pantalla no la cambia
- **Una imagen se sube al elegirla**, y solo entonces cambia el valor: si la
  subida falla, el campo conserva la anterior. Subida y no guardada queda como
  medio huérfano (Decision 024). Más de 5 MB se rechaza antes de mandar nada:
  por encima de cierto tamaño el `413` llega como error de red
- **El cliente manda un `FormData` tal cual**, sin `Content-Type`: lo escribe el
  navegador con el boundary. Serializarlo manda `"{}"`
- **Sin guardar no hay guardado, ni con Enter**: guardar lo mismo sube la
  versión y le da un `409` a quien sí estaba editando
- **Una clave sin esquema en el navegador** —la agregó un fork en el api y web
  no la trae— se muestra como JSON y no se edita

## La landing

`pages/index.vue` y `pages/[...slug].vue` montan `modules/landing/views/PaginaView.vue`,
que pide `GET /public/pages/{slug}` en SSR por la red interna. `/` es el slug
`inicio`, y `/inicio` redirige a `/` con `301` para no tener la portada en dos
URLs. La lógica que no necesita a Nuxt vive en `modules/landing/pagina.ts`.

| La API… | La landing responde |
|---|---|
| entrega la página | `200`, con `title`, `description` y Open Graph del SEO de la versión publicada |
| responde `404` o `400`, o la ruta no puede ser un slug (`/a/b`, `/Mayus`) | `404` |
| no responde, o responde `502`/`503`/`504` | `503` |
| cualquier otra cosa | `500` |

Tres trampas:

- **`useFetch` pone `statusCode: 500` también cuando la API está apagada.** Nuxt
  envuelve el fallo de red en un `NuxtError` con 500 por omisión. Lo que
  distingue "no respondió" es la causa: `cause.response` existe solo si hubo
  respuesta. Por eso `statusRespondido` y no `error.statusCode`
- **Con `curl` pelado, el error sale en JSON**: Nitro mira `Accept`. Un navegador
  o un buscador manda `text/html` y ve `error.vue`; el código es el mismo. En
  desarrollo ese JSON trae la pila
- **La clave de página es la ruta completa** (`definePageMeta({ key })`): sin
  ella, pasar de `/precios` a `/nosotros` reutiliza la vista con los datos viejos

### El marco: cabecera y pie

Toda página de la landing va dentro de `MarcoDelSitio.vue`, que pide
`GET /public/settings` y pinta la marca (logo o nombre), el menú (`site.nav`) y
el pie (`site.footer`) en SSR. La lógica pura es `modules/landing/sitio.ts`.

- **Si la configuración no llega, la página se sirve igual, sin cabecera ni
  pie**, con `200` y el fallo en el log de web (`[landing] no llego la
  configuracion del sitio`). El marco es secundario: un fallo suyo no tumba el
  contenido con un `503`
- **Los 404 y 500 de la landing también llevan el marco** (`error.vue`), para
  que haya por dónde volver. **Un 503 no**: el api no responde, y pedirle la
  configuración es esperar otro timeout —con el api apagado, 7 s más por visita
- **Los enlaces se releen a la defensiva** (`sitioDe`): solo `/ruta` o
  `https://`. El api ya los valida, pero una fila escrita a mano no pasa por el
  esquema, y lo que sale de aquí es un `href` en todas las páginas
- **El logo lleva el nombre en `alt`**: es lo que se lee y lo que se ve si la
  imagen no carga

## La capa de API

Un cliente, dos comportamientos según dónde corre:

```
navegador  → baseURL '/api/v1'                 (mismo origen, cookies solas)
servidor   → baseURL 'http://api:8080/api/v1'  (red interna del compose)
```

Son dos piezas, y no se mezclan:

- **`shared/api/useApiFetch.ts`** — lo que se renderiza en servidor (la
  landing). Solo lecturas
- **`shared/api/client.ts`** — el cliente del navegador, sobre `fetch` y sin
  dependencias. `get`, `post`, `put` (con `ifMatch` obligatorio) y `delete`.
  El reintento tras un refresh repite la petición entera, `If-Match` incluido Lo entrega `useApi()`, que **lanza en SSR** a propósito: lee
  `document.cookie`, y fallar en el sitio de la causa es mejor que un
  `document is not defined` tres llamadas más abajo

Etapas del cliente, **en este orden**:

1. **Refresh:** ante un `401`, intenta `POST /auth/refresh` **una vez** y
   reintenta la petición original. Con exclusión explícita de la ruta de refresh
   (ver `06-flujos.md`) y una sola promesa compartida para peticiones en vuelo
2. **Normalización:** todo fallo sale como `ApiError` (`shared/api/errors.ts`),
   nunca como error crudo. Un `502` de Caddy trae HTML y sale igual como
   `ApiError`, no como un `SyntaxError` de JSON

Invertir el orden deja al refresh sin la configuración original que necesita
para reintentar.

Lo que decide si un fallo se renueva, y por qué cada condición:

- **Solo un `401`.** Un `403` es un permiso que falta: renovar no lo arregla y
  rota las cookies de todas las pestañas por nada
- **Solo con la cookie `has_session`.** Sin ella no hubo sesión, y cada carga
  anónima pediría un refresh que responde `401`
- **Nunca en `/auth/refresh`, `/auth/login` ni `/auth/logout`.** El `401` del
  login son credenciales malas, no una sesión caducada
- **Una vez.** Si el reintento vuelve a dar `401`, sale el error
- **Si el refresh cae, sale la caída y no un `401`**: el guard tiene que poder
  mostrar el error en vez de mandar al login

**La carrera entre pestañas la ordena el cliente, con Web Locks**
(`navigator.locks`, candado `go-starter:refresh`). El servidor trata todo reuso
de un `rt` como robo y revoca todas las sesiones, así que dos pestañas que
renuevan a la vez con el mismo `rt` expulsarían a la persona. Con el candado, la
segunda espera; al entrar compara `XSRF-TOKEN` con el que había al mandar su
petición —el refresh lo rota— y si cambió, reintenta sin renovar.

**Esa comparación solo cuenta si había token al salir.** Tras reiniciar el
navegador, la cookie CSRF —de sesión— desaparece y `has_session` no; el primer
`GET` siembra una nueva, y compararla con "nada" parecería una renovación ajena:
se saltaría el refresh y un `rt` válido acabaría en el login.

**Cerrar sesión no lanza aunque el servidor falle.** El servidor borra las
cookies incluso si su base no responde, y el perfil se vacía antes de navegar
al login: si quedara, el botón de atrás volvería a `/admin` sin preguntar.

**El CSRF lo pone el cliente, y lo lee de la cookie en cada petición.** El
login rota el token: una copia en memoria manda el viejo y responde `403` justo
después de entrar. Y si la cookie no existe, el cliente la siembra con un
`GET /healthz` antes de la mutación — pasa siempre en `/login`, que lo sirve
Nuxt y no Go, así que abrirlo no emite la cookie.

`ApiError` distingue dos cosas que se confunden siempre:

- `answered` — hubo respuesta del servidor. Un `401` la tiene en `true`
- `unavailable` — es una caída. Un `502` sí; una petición cancelada no

Colapsarlas rompe la rehidratación de sesión: al arrancar, un `401` significa
"no hay sesión" (mostrar la landing) y una caída significa "no sé" (mostrar
error, no expulsar al usuario).

## Los cuatro estados

Toda vista que pide datos los implementa **los cuatro**, o no está hecha:

| Estado | Qué se muestra |
|---|---|
| Cargando | Esqueleto con la forma del contenido, no un spinner centrado |
| Vacío | Qué es esto y qué hacer para llenarlo. Con acción, no solo texto |
| Error | Qué pasó, botón de reintentar, y el `traceId` visible para soporte |
| Con datos | Lo normal |

El vacío es el que siempre se olvida, y es la primera pantalla que ve el dueño
de un fork recién instalado.

## Guards

- `/admin/**` exige sesión: sin ella, redirige a `/login` con `?next=`. Vive en
  `middleware/sesion.global.ts`, y la decisión en `modules/auth/sesion.ts`
  (`decidirAcceso`), que se prueba sin Nuxt
- Las secciones se ocultan por permiso leído de `me`, **como conveniencia**. La
  autorización real vive en el servidor y hay pruebas que lo confirman. Cada
  entrada del menú declara el permiso de la operación con la que su pantalla
  abre, en `modules/admin/secciones.ts`; un fork agrega ahí su dominio
- **Ocultar no protege la ruta.** Quien escribe la URL de una sección oculta
  llega a la pantalla, y lo que la niega es el `403` del servidor a su primera
  petición. Por eso la pantalla tiene un quinto estado, **sin permiso**, que dice
  qué permiso falta y no ofrece reintentar
- **Una pantalla abierta que pierde la sesión va al login ella misma**, con
  `usePedirConSesion` (`modules/auth/composables/`). El guard solo lo detecta al
  navegar, y el cliente ya renovó y reintentó antes de que llegue un `401`
- El guard cierra por omisión: es **global**, así que una ruta nueva bajo
  `/admin` está protegida sin que nadie se acuerde de protegerla
- Sin la cookie `has_session` no se pide `me`: sería un `401` que ya se sabía.
  Con ella, un `401` de `me` lleva al login y **cualquier otro fallo lleva a
  `error.vue`**, no al login: una caída no es falta de sesión
- **`?next=` solo acepta rutas de `/admin` de este origen.** Lo escribe
  cualquiera, y sin ese filtro el login es una redirección abierta: un enlace con
  el dominio de verdad que, después de pedir la contraseña, deja a la persona en
  otro sitio
- `/login` es SPA como `/admin`. Renderizado en servidor, el formulario se puede
  enviar antes de hidratar, y ese envío es el nativo del navegador: un `GET`
  que no pasa por el cliente ni por el CSRF

**El "Reintentar" de `error.vue` recarga, no navega.** `clearError({ redirect })`
hacia la ruta en la que ya está la URL es una navegación duplicada: Vue Router
la descarta sin correr ningún middleware y el error se limpia igual. El
resultado era el dashboard pintado **sin pasar por el guard**. Salió probando
la caída de verdad en un navegador; ninguna prueba unitaria lo habría visto.

## Verificación

Tres comandos, los tres obligatorios antes de un PR:

```sh
npm run test:run     # nunca 'npm test' a secas: vitest se queda en watch
npm run typecheck
npm run build        # no es redundante: resuelve los imports diferidos de rutas
```

Vitest corre en `environment: 'node'` por omisión: las pruebas de lógica pura no
necesitan DOM y arrancan antes sin él. Las de componentes piden el suyo archivo
por archivo, con `// @vitest-environment happy-dom` en la primera línea.

Todo `npm` va por `docker exec` y el workdir del contenedor es `/workspace`:

```sh
docker exec <contenedor-web> sh -c 'cd /workspace && npm run test:run'
```

**`typecheck` escupe un `ERR_PACKAGE_PATH_NOT_EXPORTED` sobre
`vue-router/volar/sfc-route-blocks` y termina en `exit=0`.** Es un desajuste
entre `vue-tsc` 3 y `vue-router` 4, cosmético: no hay ningún error de TypeScript
detrás. Lo que se mira es el código de salida, no el ruido — igual que con el
aviso de deprecación de Vite (`08-infra-local.md`).
