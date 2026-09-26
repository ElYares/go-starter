<script setup lang="ts">
// El marco del dashboard: quien tiene la sesion, la navegacion y el contenido.
// Sin nada de Nuxt, igual que los Base*: recibe el perfil y avisa de "salir".
//
// En ancho, la navegacion es un menu lateral que se oculta y se vuelve a
// mostrar, y se acuerda de como lo dejaste. En angosto no hay lateral: la
// hamburguesa abre un cajon (BaseDrawer) con las secciones y la sesion.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseButton, BaseDrawer } from '~/shared/ui'
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

// Una preferencia de quien mira, no un dato: si el navegador no deja guardar
// (ventana privada, almacenamiento bloqueado), el menu arranca visible y ya.
const CLAVE_PLEGADO = 'admin.menu-plegado'

function leerPlegado(): boolean {
  try {
    return localStorage.getItem(CLAVE_PLEGADO) === '1'
  } catch {
    return false
  }
}

const plegado = ref(leerPlegado())

watch(plegado, (valor) => {
  try {
    if (valor) localStorage.setItem(CLAVE_PLEGADO, '1')
    else localStorage.removeItem(CLAVE_PLEGADO)
  } catch {
    // Sin almacenamiento, la preferencia dura lo que dure la pestana.
  }
})

const cajonAbierto = ref(false)

// El mismo corte que el CSS de abajo. Si la ventana se ensancha con el cajon
// abierto, el cajon se cierra: en ancho ya esta el lateral, y quedaria el velo
// tapando la pagina sin hamburguesa a la vista para quitarlo.
const ANGOSTO = '(max-width: 48rem)'
let consulta: MediaQueryList | undefined

function alCambiarElAncho(e: MediaQueryListEvent) {
  if (!e.matches) cajonAbierto.value = false
}

onMounted(() => {
  if (typeof window.matchMedia !== 'function') return
  consulta = window.matchMedia(ANGOSTO)
  consulta.addEventListener('change', alCambiarElAncho)
})

onBeforeUnmount(() => consulta?.removeEventListener('change', alCambiarElAncho))
</script>

<template>
  <div :class="['shell', { plegado }]">
    <aside id="menu-lateral" class="lateral">
      <span class="marca">go-starter</span>
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
    </aside>

    <div class="columna">
      <header class="barra">
        <BaseButton
          class="alternar"
          variant="ghost"
          size="sm"
          aria-controls="menu-lateral"
          :aria-expanded="!plegado"
          :aria-label="plegado ? 'Mostrar menu' : 'Ocultar menu'"
          @click="plegado = !plegado"
        >
          <svg class="icono" viewBox="0 0 24 24" aria-hidden="true">
            <rect x="3" y="4" width="18" height="16" rx="2" />
            <path d="M9 4v16" />
          </svg>
        </BaseButton>

        <BaseDrawer v-model:open="cajonAbierto" title="go-starter" close-label="Cerrar menu">
          <template #trigger>
            <BaseButton class="hamburguesa" variant="ghost" size="sm" aria-label="Abrir menu">
              <svg class="icono" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M4 6h16M4 12h16M4 18h16" />
              </svg>
            </BaseButton>
          </template>

          <!-- Cada enlace cierra el cajon: el shell no se vuelve a montar
               al cambiar de seccion, y el cajon se quedaria abierto
               tapando la pantalla a la que se acaba de ir. -->
          <nav class="nav" aria-label="Secciones del dashboard">
            <RouterLink
              v-for="s in visibles"
              :key="s.ruta"
              :to="s.ruta"
              class="enlace"
              exact-active-class="actual"
              @click="cajonAbierto = false"
            >
              {{ s.etiqueta }}
            </RouterLink>
          </nav>

          <div class="cajon-sesion">
            <span class="quien">Sesion de {{ perfil.displayName }}</span>
            <RouterLink to="/admin/contrasena" class="mi-contrasena" @click="cajonAbierto = false">
              Mi contrasena
            </RouterLink>
            <BaseButton variant="secondary" size="sm" :loading="saliendo" @click="$emit('salir')">
              Cerrar sesion
            </BaseButton>
          </div>
        </BaseDrawer>

        <!-- La marca vive en el lateral. Arriba solo aparece cuando no hay
             lateral a la vista: plegado, o en angosto. -->
        <span class="marca marca-arriba">go-starter</span>

        <div class="sesion">
          <span class="quien" :title="perfil.displayName">Sesion de {{ perfil.displayName }}</span>
          <RouterLink to="/admin/contrasena" class="mi-contrasena">Mi contrasena</RouterLink>
          <BaseButton variant="secondary" size="sm" :loading="saliendo" @click="$emit('salir')">
            Cerrar sesion
          </BaseButton>
        </div>
      </header>

      <main class="contenido">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
/* Dos columnas: el lateral y todo lo demas. Plegado, el lateral sale del grid
   y el contenido toma el ancho entero. */
.shell {
  display: grid;
  grid-template-columns: 15rem minmax(0, 1fr);
  min-height: 100vh;
}
.shell.plegado {
  grid-template-columns: minmax(0, 1fr);
}
.shell.plegado .lateral {
  display: none;
}

/* El lateral se queda quieto mientras el contenido se desplaza, y si las
   secciones de un fork no caben en el alto, se desplaza el solo. */
.lateral {
  position: sticky;
  top: 0;
  height: 100vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--color-border);
  background: var(--color-surface);
}
.lateral .marca {
  display: flex;
  align-items: center;
  min-height: 3.5rem;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.marca {
  font-weight: 700;
  white-space: nowrap;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-3);
}
.enlace {
  padding: var(--space-2) var(--space-3);
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
  background: var(--color-surface-hover);
  color: var(--color-accent-strong);
  font-weight: 600;
}

.barra {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 3.5rem;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}
.barra .hamburguesa,
.marca-arriba {
  display: none;
}
.shell.plegado .marca-arriba {
  display: inline;
}

/* La sesion va a la derecha, y de ella solo cede el nombre: un displayName
   largo se recorta con "..." en vez de empujar los botones. */
.sesion {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
  margin-left: auto;
}
.quien {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.mi-contrasena {
  white-space: nowrap;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.sesion .boton {
  white-space: nowrap;
}

.icono {
  width: 1.25rem;
  height: 1.25rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.contenido {
  width: 100%;
  max-width: 56rem;
  margin: 0 auto;
  padding: var(--space-6) var(--space-4);
}

/* La sesion, al pie del cajon del angosto. */
.cajon-sesion {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
  margin-top: auto;
  padding: var(--space-4);
  border-top: 1px solid var(--color-border);
}
.cajon-sesion .quien {
  max-width: 100%;
}

/* En angosto no hay lateral ni sesion arriba: la hamburguesa y la marca, y el
   resto dentro del cajon. */
@media (max-width: 48rem) {
  .shell,
  .shell.plegado {
    grid-template-columns: minmax(0, 1fr);
  }
  .lateral,
  .barra .alternar,
  .sesion {
    display: none;
  }
  .barra .hamburguesa {
    display: inline-flex;
  }
  .marca-arriba {
    display: inline;
  }
}
</style>
