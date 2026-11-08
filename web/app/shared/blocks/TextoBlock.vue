<script setup lang="ts">
import { computed } from 'vue'
import type { TextoContenido } from './tipos'
import BandaDeSeccion from './BandaDeSeccion.vue'

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
  <BandaDeSeccion tono="suave">
    <div class="texto">
      <h2 v-if="contenido.title">{{ contenido.title }}</h2>
      <p v-for="(parrafo, i) in parrafos" :key="i">{{ parrafo }}</p>
    </div>
  </BandaDeSeccion>
</template>

<style scoped>
.texto {
  max-width: var(--measure);
  margin-inline: auto;
  line-height: 1.7;
}
h2 {
  margin: 0 0 var(--space-4);
  font-size: var(--text-xl);
  line-height: 1.2;
  overflow-wrap: anywhere;
}
p {
  margin: 0 0 var(--space-4);
}
</style>
