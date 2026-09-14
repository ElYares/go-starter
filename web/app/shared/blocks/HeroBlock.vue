<script setup lang="ts">
import type { HeroContenido } from './tipos'

// La portada de una pagina. El enlace es un <a> y no un NuxtLink a proposito:
// la landing tiene que funcionar sin JavaScript, y un bloque que no importa
// nada de Nuxt se prueba sin su arnes.
defineProps<{ contenido: HeroContenido }>()
</script>

<template>
  <section class="hero">
    <h1>{{ contenido.title }}</h1>
    <p v-if="contenido.subtitle" class="bajada">{{ contenido.subtitle }}</p>
    <a v-if="contenido.cta" class="cta" :href="contenido.cta.href">{{ contenido.cta.label }}</a>
  </section>
</template>

<style scoped>
.hero {
  padding: var(--space-8) 0;
}
h1 {
  margin: 0 0 var(--space-4);
  font-size: var(--text-display);
  line-height: 1.1;
}
.bajada {
  margin: 0 0 var(--space-6);
  font-size: var(--text-lg);
  color: var(--color-text-muted);
}
.cta {
  display: inline-block;
  padding: var(--space-3) var(--space-6);
  border-radius: var(--radius-md);
  background: var(--color-accent);
  color: var(--color-on-accent);
  font-weight: 600;
  text-decoration: none;
}
.cta:hover {
  background: var(--color-accent-strong);
}
.cta:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
