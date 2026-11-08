<script setup lang="ts">
import type { HeroContenido } from './tipos'
import { esHrefSeguro } from './href'
import BandaDeSeccion from './BandaDeSeccion.vue'

// La portada de una pagina. El enlace es un <a> y no un NuxtLink a proposito:
// la landing tiene que funcionar sin JavaScript, y un bloque que no importa
// nada de Nuxt se prueba sin su arnes.
//
// Un enlace que no pasa esHrefSeguro no se pinta: el boton desaparece antes que
// mandar a quien visita la portada a otro dominio.
defineProps<{ contenido: HeroContenido }>()
</script>

<template>
  <BandaDeSeccion tono="marca" amplia>
    <div class="hero">
      <h1>{{ contenido.title }}</h1>
      <p v-if="contenido.subtitle" class="bajada">{{ contenido.subtitle }}</p>
      <a v-if="contenido.cta && esHrefSeguro(contenido.cta.href)" class="cta" :href="contenido.cta.href">{{ contenido.cta.label }}</a>
    </div>
  </BandaDeSeccion>
</template>

<style scoped>
.hero {
  text-align: center;
}
h1 {
  margin: 0 auto var(--space-4);
  max-width: 20ch;
  font-size: var(--text-hero);
  line-height: 1.1;
  overflow-wrap: anywhere;
}
.bajada {
  margin: 0 auto var(--space-8);
  max-width: 40rem;
  font-size: var(--text-lg);
  color: var(--color-on-brand-muted);
}
.cta {
  display: inline-flex;
  align-items: center;
  min-height: 2.75rem;
  padding: var(--space-3) var(--space-8);
  border-radius: var(--radius-full);
  background: var(--color-on-brand);
  color: var(--color-brand);
  font-weight: 700;
  text-decoration: none;
}
.cta:hover {
  background: var(--color-on-brand-muted);
}
.cta:focus-visible {
  outline: var(--focus-ring) solid var(--color-focus-on-brand);
  outline-offset: 3px;
}
</style>
