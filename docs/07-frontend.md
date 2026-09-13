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
│   └── auth/                 # login, store de sesión, guards
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
  17 pares en los dos temas contra 4.5:1. Un fondo nuevo sin su par de texto no
  es un olvido: es un par que falta en esa lista
- `test/tokens-sin-literales.spec.ts` falla si un `.vue` de `app/` escribe un
  color, un espacio o un radio a mano. El grosor de un borde (`1px`) sí se
  permite: no es un token en ningún sistema, e inventar `--border-1` sería
  ceremonia que nadie cambia

## Bloques del CMS

Un tipo de bloque son dos cosas que viajan juntas: un componente y su esquema.

```ts
// shared/blocks/registry.ts
export const blocks = {
  hero:     { component: () => import('./HeroBlock.vue'),     label: 'Portada' },
  features: { component: () => import('./FeaturesBlock.vue'), label: 'Características' },
  cta:      { component: () => import('./CtaBlock.vue'),      label: 'Llamado a la acción' },
}
```

- El renderizador recorre `page.blocks` y resuelve `type` contra el registro
- Un `type` desconocido **no revienta**: cae a un componente vacío que registra
  el aviso. Un fork que borra un bloque no puede tumbar páginas ya publicadas
- El editor lee el mismo registro para ofrecer "agregar bloque". Una sola fuente
- Cambiar el lenguaje visual de un fork es escribir componentes de bloque
  nuevos, no tocar el editor ni el backend

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
  dependencias. Lo entrega `useApi()`, que **lanza en SSR** a propósito: lee
  `document.cookie`, y fallar en el sitio de la causa es mejor que un
  `document is not defined` tres llamadas más abajo

Etapas del cliente, **en este orden**:

1. **Refresh:** ante un `401`, intenta `POST /auth/refresh` **una vez** y
   reintenta la petición original. Con exclusión explícita de la ruta de refresh
   (ver `06-flujos.md`) y una sola promesa compartida para peticiones en vuelo.
   **Todavía no existe:** llega con CU-002, cuando exista el endpoint
2. **Normalización:** todo fallo sale como `ApiError` (`shared/api/errors.ts`),
   nunca como error crudo. Un `502` de Caddy trae HTML y sale igual como
   `ApiError`, no como un `SyntaxError` de JSON

Invertir el orden deja al refresh sin la configuración original que necesita
para reintentar.

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
- Las acciones se ocultan por permiso leído de `me`, **como conveniencia**. La
  autorización real vive en el servidor y hay pruebas que lo confirman
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
