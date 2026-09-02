# 10 · Roadmap

Dos reglas que ordenan todo lo demás:

- **Una fase, una incógnita.** Si una fase prueba dos cosas nuevas y falla, no
  sabes cuál fue
- **Lo riesgoso temprano.** El cableado de SSR, el registro de módulos y el
  contenido versionado van antes que cualquier CRUD: el CRUD ya sabes hacerlo,
  y esas tres pueden obligarte a rehacer decisiones

Cada fase termina con un criterio **verificable desde fuera**, no con "está
avanzado".

---

## Fase 0 — Que levante

Compose con `edge`, `api`, `web`, `db`. Go sirviendo `/healthz`, Nuxt sirviendo
una página, ambos por el mismo origen. `.env` sin defaults para secretos. CI con
los tres trabajos, aunque casi no haya qué probar.

**Hecho cuando:** `devherd up` y `curl http://go-starter.localhost/` devuelve
HTML y `curl http://go-starter.localhost/api/v1/healthz` devuelve `200`, sin
ningún puerto publicado en el host, y el CI pasa en un PR.

> Incógnita: el cableado. Nada de negocio todavía.

---

## Fase 1 — El molde

Lo que todo lo demás va a copiar: `httpx` con problem+json y `traceId`, `paging`
con tope y lista blanca, `audit`, el registro de módulos con su prueba de
límites, `openapi.yaml` con generación en los dos lados, y en el front los
tokens más los primitivos de `shared/ui/`.

**Hecho cuando:** existe **un** recurso de juguete que cumple el checklist
completo de `04-reglas-de-crud.md` §8, incluida la prueba de que
`?size=1000000` devuelve 100 y la de que un `sort` inválido da `400`. Y una
prueba que falla si `platform` importa un módulo.

> Incógnita: si el molde aguanta. Se descubre con un consumidor, no con cero.

---

## Fase 2 — Identidad y sesión

`users`, `roles`, `permissions` sembrados desde los módulos, `refresh_tokens`
con rotación y `replaced_by`. Login, refresh, logout, `me`. En el front: pantalla
de login, store de sesión, interceptor de refresh, guard de `/admin`,
rehidratación al cargar.

**Hecho cuando:** entras al dashboard, esperas a que expire el `at`, sigues
navegando sin volver a autenticarte, y reusar un `rt` viejo tumba la sesión
entera.

> Incógnita: la rotación y su carrera entre pestañas.

---

## Fase 3 — La landing editable

`pages` y `page_versions`, CRUD completo con `If-Match`, `publish`, el endpoint
público por slug. En el front: el registro de bloques, el renderizador SSR y el
editor en el dashboard.

**Hecho cuando:** cambias el texto de la portada desde `/admin`, publicas, y el
HTML que devuelve `curl http://go-starter.localhost/` —sin JavaScript— ya trae
el texto nuevo. Y publicar la versión anterior lo revierte.

> Incógnita: el contenido versionado y el SSR contra la red interna. Es la fase
> que define si el starter sirve de verdad.

---

## Fase 4 — Medios y configuración del sitio

Subida con streaming, deduplicación por SHA-256, tipo deducido de los bytes.
`settings` con esquema por clave y separación público/privado. Marca,
navegación y pie editables desde el dashboard.

**Hecho cuando:** subes un logo desde el dashboard, aparece en la landing
publicada, y subir el mismo archivo dos veces no crea dos registros.

---

## Fase 5 — Cerrar el ciclo del fork

Seed de desarrollo, `11-forks.md` probado de verdad: crear un módulo nuevo
siguiendo la receta y borrar uno existente, comprobando que el proyecto compila
y el CI pasa en ambos casos.

**Hecho cuando:** un fork nuevo llega a "landing propia publicada" en menos de
una hora, y esa hora está medida, no estimada.

---

## Después, según el fork

No son fases del starter; son puntos de partida documentados:

- **Tienda:** módulo `catalog` (productos, categorías, variantes) + `orders`
- **Landing:** ya está completo en la fase 4. Solo se cambian bloques y tokens
- **Administrativo:** agregación **en SQL**, nunca en el navegador, y contrato de
  series en vez de filas. Graficar 200 mil filas en el cliente no se puede
