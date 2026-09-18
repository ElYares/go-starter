<script setup lang="ts">
// La cabecera de la landing: la marca y el menu. Recibe el sitio ya armado; no
// pide nada. El logo, si hay, lleva el nombre en `alt`: es lo que lee un
// lector de pantalla y lo que se ve si la imagen no carga.
import type { Sitio } from '../sitio'

defineProps<{
  sitio: Sitio
  /** La URL publica de una imagen, a partir de su id. */
  urlDeLogo: (id: string) => string
}>()
</script>

<template>
  <header class="cabecera">
    <NuxtLink to="/" class="marca">
      <img v-if="sitio.logo" :src="urlDeLogo(sitio.logo)" :alt="sitio.nombre" class="logo" />
      <span v-else class="nombre">{{ sitio.nombre }}</span>
    </NuxtLink>
    <nav v-if="sitio.nav.length" aria-label="Principal">
      <ul class="menu">
        <li v-for="(enlace, i) in sitio.nav" :key="i">
          <NuxtLink :to="enlace.href">{{ enlace.label }}</NuxtLink>
        </li>
      </ul>
    </nav>
  </header>
</template>

<style scoped>
.cabecera {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3) var(--space-6);
  align-items: center;
  justify-content: space-between;
  max-width: 60rem;
  margin: 0 auto;
  padding: var(--space-4);
}
.marca {
  display: inline-flex;
  align-items: center;
  color: var(--color-text);
  text-decoration: none;
}
.logo {
  display: block;
  max-height: 2.5rem;
  max-width: 12rem;
  object-fit: contain;
}
.nombre {
  font-size: var(--text-lg);
  font-weight: 700;
}
.menu {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}
.menu a {
  color: var(--color-text-muted);
  text-decoration: none;
}
.menu a:hover,
.menu a.router-link-exact-active {
  color: var(--color-text);
}
</style>
