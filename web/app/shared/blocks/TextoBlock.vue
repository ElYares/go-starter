<script setup lang="ts">
import { computed } from 'vue'
import type { TextoContenido } from './tipos'

const props = defineProps<{ contenido: TextoContenido }>()

// Texto plano, no HTML ni Markdown: lo dice el esquema del catalogo. Los saltos
// de linea separan parrafos, y cada parrafo se pinta con interpolacion, nunca
// con v-html. Lo que escribe quien edita no puede inyectar marcado en la
// landing.
const parrafos = computed(() =>
  props.contenido.body
    .split(/\n+/)
    .map((p) => p.trim())
    .filter(Boolean),
)
</script>

<template>
  <section class="texto">
    <h2 v-if="contenido.title">{{ contenido.title }}</h2>
    <p v-for="(parrafo, i) in parrafos" :key="i">{{ parrafo }}</p>
  </section>
</template>

<style scoped>
.texto {
  padding: var(--space-6) 0;
}
h2 {
  margin: 0 0 var(--space-4);
  font-size: var(--text-lg);
}
p {
  margin: 0 0 var(--space-3);
}
</style>
