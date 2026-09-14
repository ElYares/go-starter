<script setup lang="ts">
import type { FeaturesContenido } from './tipos'

defineProps<{ contenido: FeaturesContenido }>()
</script>

<template>
  <section class="features">
    <h2 v-if="contenido.title">{{ contenido.title }}</h2>
    <ul>
      <li v-for="(item, i) in contenido.items" :key="i">
        <h3>{{ item.title }}</h3>
        <p v-if="item.text">{{ item.text }}</p>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.features {
  padding: var(--space-6) 0;
}
h2 {
  margin: 0 0 var(--space-4);
  font-size: var(--text-lg);
}
ul {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}
li {
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-base);
}
p {
  margin: 0;
  color: var(--color-text-muted);
}
</style>
