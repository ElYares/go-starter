<script setup lang="ts">
// El pie de la landing: su texto y sus enlaces. Si no hay ninguno de los dos,
// no pinta nada: un <footer> vacio es ruido para un lector de pantalla.
import type { Sitio } from '../sitio'

defineProps<{ sitio: Sitio }>()
</script>

<template>
  <footer v-if="sitio.pie.texto || sitio.pie.enlaces.length" class="pie">
    <div class="interior">
    <p v-if="sitio.pie.texto" class="texto">{{ sitio.pie.texto }}</p>
    <nav v-if="sitio.pie.enlaces.length" aria-label="Pie de pagina">
      <ul class="enlaces">
        <li v-for="(enlace, i) in sitio.pie.enlaces" :key="i">
          <NuxtLink :to="enlace.href">{{ enlace.label }}</NuxtLink>
        </li>
      </ul>
    </nav>
    </div>
  </footer>
</template>

<style scoped>
.pie {
  border-top: 1px solid var(--color-border);
  background: var(--color-surface);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.interior {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3) var(--space-6);
  align-items: center;
  justify-content: space-between;
  max-width: var(--container-w);
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
}
.texto {
  margin: 0;
}
.enlaces {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}
.enlaces a {
  color: inherit;
}
</style>
