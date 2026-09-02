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
```

Ninguna vista importa Reka UI directamente. Si lo hiciera, cambiar de librería
—o quitarla— sería tocar cincuenta archivos en vez de diez.

### Tokens: lo que un fork cambia primero

```css
/* assets/tokens/base.css */
:root {
  --color-bg: …;         --color-surface: …;
  --color-text: …;       --color-text-muted: …;
  --color-accent: …;     --color-accent-strong: …;  --color-on-accent: …;
  --color-border: …;     --color-danger: …;
  --font-sans: …;        --radius-md: …;            --space-4: …;
}
```

Dos reglas que ya costaron caro en el proyecto hermano:

- **Un color de marca no sirve automáticamente para texto.** Un verde de marca
  puede dar 2.5:1 de contraste. Por eso existe `--color-accent-strong` aparte:
  el de marca pinta rellenos, el fuerte pinta texto y bordes
- **Todo token de relleno necesita su par de texto.** `--color-on-accent` existe
  como token, y no como un `#fff` dentro de un botón, porque en tema oscuro el
  par se invierte

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

Interceptores, **en este orden**:

1. **Refresh:** ante un `401`, intenta `POST /auth/refresh` **una vez** y
   reintenta la petición original. Con exclusión explícita de la ruta de refresh
   (ver `06-flujos.md`) y una sola promesa compartida para peticiones en vuelo
2. **Normalización:** todo fallo sale como `ApiError`, nunca como error crudo

Invertir el orden deja al refresh sin la configuración original que necesita
para reintentar.

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

- `/admin/**` exige sesión: sin ella, redirige al login con `?next=`
- Las acciones se ocultan por permiso leído de `me`, **como conveniencia**. La
  autorización real vive en el servidor y hay pruebas que lo confirman
- El guard cierra por omisión: una ruta nueva bajo `/admin` está protegida sin
  que nadie se acuerde de protegerla

## Verificación

Tres comandos, los tres obligatorios antes de un PR:

```sh
npm run test:run     # nunca 'npm test' a secas: vitest se queda en watch
npm run typecheck
npm run build        # no es redundante: resuelve los imports diferidos de rutas
```

Todo `npm` va por `docker exec` y el workdir del contenedor es `/workspace`:

```sh
docker exec <contenedor-web> sh -c 'cd /workspace && npm run test:run'
```

**`typecheck` escupe un `ERR_PACKAGE_PATH_NOT_EXPORTED` sobre
`vue-router/volar/sfc-route-blocks` y termina en `exit=0`.** Es un desajuste
entre `vue-tsc` 3 y `vue-router` 4, cosmético: no hay ningún error de TypeScript
detrás. Lo que se mira es el código de salida, no el ruido — igual que con el
aviso de deprecación de Vite (`08-infra-local.md`).
