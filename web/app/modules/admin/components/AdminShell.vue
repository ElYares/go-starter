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
    <header class="barra">
      <span class="marca">go-starter</span>
      <div class="sesion">
        <span class="quien">Sesion de {{ perfil.displayName }}</span>
        <RouterLink to="/admin/contrasena" class="mi-contrasena">Mi contrasena</RouterLink>
        <BaseButton variant="secondary" size="sm" :loading="saliendo" @click="$emit('salir')">
          Cerrar sesion
        </BaseButton>
      </div>
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
.barra {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}
.marca {
  font-weight: 700;
}
.sesion {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
.mi-contrasena {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.quien {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
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
