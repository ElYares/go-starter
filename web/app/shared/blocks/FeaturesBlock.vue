<script setup lang="ts">
import type { FeaturesContenido } from './tipos'
import BandaDeSeccion from './BandaDeSeccion.vue'

defineProps<{ contenido: FeaturesContenido }>()
</script>

<template>
  <BandaDeSeccion>
    <h2 v-if="contenido.title">{{ contenido.title }}</h2>
    <ul>
      <li v-for="(item, i) in contenido.items" :key="i">
        <span class="orden" aria-hidden="true">{{ i + 1 }}</span>
        <h3>{{ item.title }}</h3>
        <p v-if="item.text">{{ item.text }}</p>
      </li>
    </ul>
  </BandaDeSeccion>
</template>

<style scoped>
h2 {
  margin: 0 0 var(--space-8);
  font-size: var(--text-2xl);
  line-height: 1.2;
  text-align: center;
  overflow-wrap: anywhere;
}
ul {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 15rem), 1fr));
  gap: var(--space-6);
  margin: 0 auto;
  padding: 0;
  max-width: 100%;
  list-style: none;
}
li {
  padding: var(--space-6);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.orden {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  margin-bottom: var(--space-3);
  border-radius: var(--radius-full);
  background: var(--color-accent);
  color: var(--color-on-accent);
  font-size: var(--text-sm);
  font-weight: 700;
}
h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-lg);
  overflow-wrap: anywhere;
}
p {
  margin: 0;
  color: var(--color-text-muted);
}
</style>
