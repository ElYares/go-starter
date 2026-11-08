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
    <a class="saltar" href="#contenido">Saltar al contenido</a>
    <div class="interior">
      <NuxtLink to="/" class="marca">
        <img v-if="sitio.logo" :src="urlDeLogo(sitio.logo)" :alt="sitio.nombre" class="logo" />
        <span v-else class="nombre">{{ sitio.nombre }}</span>
      </NuxtLink>
      <nav v-if="sitio.nav.length" aria-label="Principal">
        <!-- Escritorio: el menu a la vista. -->
        <ul class="menu escritorio">
          <li v-for="(enlace, i) in sitio.nav" :key="i">
            <NuxtLink :to="enlace.href">{{ enlace.label }}</NuxtLink>
          </li>
        </ul>
        <!-- Movil: un <details> nativo, que abre y cierra sin JavaScript. -->
        <details class="movil">
          <summary>Menú</summary>
          <ul class="menu">
            <li v-for="(enlace, i) in sitio.nav" :key="i">
              <NuxtLink :to="enlace.href">{{ enlace.label }}</NuxtLink>
            </li>
          </ul>
        </details>
      </nav>
    </div>
  </header>
</template>

<style scoped>
.cabecera {
  position: sticky;
  top: 0;
  z-index: 10;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg);
}
@supports (backdrop-filter: blur(1px)) {
  .cabecera {
    background: color-mix(in srgb, var(--color-bg) 88%, transparent);
    backdrop-filter: blur(8px);
  }
}
.saltar {
  position: absolute;
  left: var(--space-4);
  top: var(--space-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  background: var(--color-accent);
  color: var(--color-on-accent);
  transform: translateY(-200%);
}
.saltar:focus-visible {
  transform: none;
}
.interior {
  position: relative;
  display: flex;
  gap: var(--space-3) var(--space-6);
  align-items: center;
  justify-content: space-between;
  max-width: var(--container-w);
  min-height: var(--header-h);
  margin: 0 auto;
  padding: var(--space-2) var(--space-4);
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
/* El enlace activo no depende solo del color. */
.menu a.router-link-exact-active {
  text-decoration: underline;
  text-decoration-thickness: 2px;
  text-underline-offset: 6px;
  text-decoration-color: var(--color-accent-strong);
}
.escritorio {
  display: none;
}
summary {
  display: inline-flex;
  align-items: center;
  min-width: 2.75rem;
  min-height: 2.75rem;
  padding-inline: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  cursor: pointer;
  list-style: none;
}
summary::-webkit-details-marker {
  display: none;
}
.movil .menu {
  position: absolute;
  inset: 100% var(--space-4) auto;
  flex-direction: column;
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  box-shadow: var(--shadow-card);
}
.movil .menu a {
  display: block;
  padding-block: var(--space-2);
}
@media (min-width: 40rem) {
  .escritorio {
    display: flex;
  }
  .movil {
    display: none;
  }
}
</style>
