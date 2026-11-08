<script setup lang="ts">
import { RenderDeBloques } from '~/shared/blocks'
import MarcoDelSitio from '../components/MarcoDelSitio.vue'
import { usePaginaPublica } from '../composables/usePaginaPublica'
import { metaDePagina, SLUG_DE_PORTADA, slugDeRuta } from '../pagina'

// Una pagina del CMS, renderizada en servidor. El HTML que sale ya trae el
// texto y las meta etiquetas: la landing tiene que verse igual con JavaScript
// apagado, porque asi la ven los buscadores y los previews. CU-005.
const props = defineProps<{ segmentos?: string | string[] }>()

const slug = slugDeRuta(props.segmentos)
if (slug === null) {
  throw createError({ statusCode: 404, statusMessage: 'Pagina no encontrada', data: { landing: true }, fatal: true })
}

// `/inicio` y `/` serian la misma pagina en dos URLs, y un buscador las
// indexaria como contenido duplicado. La portada vive en `/`.
if (props.segmentos !== undefined && slug === SLUG_DE_PORTADA) {
  await navigateTo('/', { redirectCode: 301 })
}

const pagina = await usePaginaPublica(slug)

useSeoMeta(metaDePagina(pagina.value, useRequestURL().href))
</script>

<template>
  <MarcoDelSitio>
    <main id="contenido" class="landing">
      <RenderDeBloques :bloques="pagina.blocks" />
      <section v-if="!pagina.blocks.length" class="vacia">
        <h1>Esta página aún no tiene contenido</h1>
        <p>Vuelve en unos días, o ve al inicio.</p>
        <a href="/">Ir al inicio</a>
      </section>
    </main>
  </MarcoDelSitio>
</template>

<style scoped>
/* Sin ancho ni relleno: cada bloque es una banda que llega al borde. */
.landing {
  display: block;
}
.vacia {
  max-width: var(--measure);
  margin-inline: auto;
  padding: var(--space-24) var(--space-4);
  text-align: center;
}
</style>
