<script setup lang="ts">
// El marco del dashboard: quien tiene la sesion, la navegacion y el contenido.
// Sin nada de Nuxt, igual que los Base*: recibe el perfil y avisa de "salir".
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseButton } from '~/shared/ui'
import type { Perfil } from '~/modules/auth/sesion'
import { seccionesVisibles, SECCIONES, type Seccion } from '../secciones'

const props = withDefaults(
  defineProps<{
    perfil: Perfil
    saliendo?: boolean
    secciones?: Seccion[]
  }>(),
  { saliendo: false, secciones: () => SECCIONES },
)

defineEmits<{ salir: [] }>()

const visibles = computed(() => seccionesVisibles(props.perfil.permissions, props.secciones))
</script>

<template>
  <div class="shell">
    <!-- Sin envoltorio para la sesion: cada pieza es una celda del grid, y en
         angosto "Cerrar sesion" sube junto a la marca. -->
    <header class="barra">
      <span class="marca">go-starter</span>
      <span class="quien" :title="perfil.displayName">Sesion de {{ perfil.displayName }}</span>
      <RouterLink to="/admin/contrasena" class="mi-contrasena">Mi contrasena</RouterLink>
      <BaseButton
        class="salir"
        variant="secondary"
        size="sm"
        :loading="saliendo"
        @click="$emit('salir')"
      >
        Cerrar sesion
      </BaseButton>
    </header>

    <nav class="nav" aria-label="Secciones del dashboard">
      <!-- exact-active y no active: con `active`, Inicio (/admin) quedaria
           marcado en todas las secciones, porque todas cuelgan de /admin. -->
      <RouterLink
        v-for="s in visibles"
        :key="s.ruta"
        :to="s.ruta"
        class="enlace"
        exact-active-class="actual"
      >
        {{ s.etiqueta }}
      </RouterLink>
    </nav>

    <main class="contenido">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
}
/* Una fila en ancho. El nombre es lo unico que cede: un displayName largo se
   recorta con "..." en vez de partir la marca o los botones en dos lineas. */
.barra {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  grid-template-areas: 'marca quien contrasena salir';
  align-items: center;
  column-gap: var(--space-3);
  row-gap: var(--space-1);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}
.marca {
  grid-area: marca;
  font-weight: 700;
  white-space: nowrap;
}
.quien {
  grid-area: quien;
  justify-self: end;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.mi-contrasena {
  grid-area: contrasena;
  white-space: nowrap;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.salir {
  grid-area: salir;
  white-space: nowrap;
}

/* En angosto no cabe en una fila: arriba la marca y "Cerrar sesion", abajo
   quien tiene la sesion y "Mi contrasena". */
@media (max-width: 48rem) {
  .barra {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      'marca salir'
      'quien contrasena';
  }
  .quien {
    justify-self: start;
  }
}
.nav {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--color-border);
}
.enlace {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--color-text);
  text-decoration: none;
  font-size: var(--text-sm);
}
.enlace:hover {
  background: var(--color-surface-hover);
}
.enlace:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: var(--focus-ring);
}
.actual {
  color: var(--color-accent-strong);
  font-weight: 600;
}
.contenido {
  max-width: 56rem;
  margin: 0 auto;
  padding: var(--space-6) var(--space-4);
}
</style>
