<script setup lang="ts">
// La configuracion del sitio, en sus estados de docs/07-frontend.md mas el de
// "sin permiso". Sin nada de Nuxt: recibe `cargar` y no sabe de donde salen los
// datos, asi se prueba cada estado con una promesa a mano.
//
// Una tarjeta por clave, dicha para quien administra el sitio: el nombre y la
// descripcion de su esquema, y el valor resumido ("Nombre: go-starter", el
// color como muestra). La clave tecnica queda chica al pie, para desarrollo.
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseBadge, BaseButton, BaseEmptyState } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import type { Esquema } from '~/shared/formularios/esquema'
import { esquemas as esquemasDelApi } from '../esquemas'
import { fecha, resumir } from '../resumen'

const props = withDefaults(
  defineProps<{
    cargar: () => Promise<Schemas['SettingsPage']>
    esquemas?: Record<string, Esquema>
  }>(),
  { esquemas: () => esquemasDelApi },
)

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const pagina = ref<Schemas['SettingsPage'] | null>(null)
const fallo = ref<ApiError | null>(null)

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    pagina.value = await props.cargar()
    estado.value = 'listo'
  } catch (causa) {
    // Lo que no es de la API es un bug: sube, no se disfraza de "no se pudo".
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

onMounted(traer)

// 403 es "el servidor sabe quien eres y no te deja". Es la autorizacion real de
// CU-003: la entrada del menu solo se oculta, y quien escribe la URL llega aqui.
const sinPermiso = computed(() => fallo.value?.status === 403)

const tituloError = computed(() => {
  if (sinPermiso.value) return 'No tienes permiso para ver la configuracion'
  if (fallo.value?.unavailable) return 'El servidor no responde'
  return 'No se pudo cargar la configuracion'
})

const descripcionError = computed(() =>
  sinPermiso.value
    ? 'Pidele a quien administra el sitio el permiso settings.read.'
    : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.',
)

const tarjetas = computed(() =>
  (pagina.value?.content ?? []).map((s) => {
    const esquema = props.esquemas[s.key]
    const lineas = resumir(esquema, s.value)
    return {
      clave: s.key,
      titulo: esquema?.title ?? s.key,
      descripcion: esquema?.description,
      lineas,
      // Una lista entera (la navegacion) es una sola linea sin etiqueta: va
      // como parrafo, no como un par nombre-valor al que le falta el nombre.
      conEtiquetas: lineas.some((l) => l.etiqueta),
      publica: s.isPublic,
      actualizada: fecha(s.updatedAt),
    }
  }),
)

const faltan = computed(() => {
  const p = pagina.value
  return p ? p.page.totalElements - p.content.length : 0
})
</script>

<template>
  <section class="settings" :aria-busy="estado === 'cargando' || undefined">
    <h1>Configuracion</h1>

    <!-- Cargando: la forma de la tabla, no un spinner centrado. -->
    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando la configuracion">
      <div v-for="n in 4" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState :title="tituloError" :description="descripcionError">
        <template #accion>
          <BaseButton v-if="!sinPermiso" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="referencia">
        Referencia: <code>{{ fallo.traceId }}</code>
      </p>
    </div>

    <div v-else data-estado="listo">
      <BaseEmptyState
        v-if="tarjetas.length === 0"
        title="Todavia no hay configuracion"
        description="Las claves se crean con POST /api/v1/settings, con el permiso settings.write."
      >
        <template #accion>
          <BaseButton variant="secondary" @click="traer">Volver a cargar</BaseButton>
        </template>
      </BaseEmptyState>

      <template v-else>
        <p class="intro">Lo que muestra la landing. Elige un ajuste para editarlo.</p>

        <ul class="tarjetas">
          <li v-for="t in tarjetas" :key="t.clave" class="tarjeta">
            <div class="cabeza">
              <!-- El enlace cubre la tarjeta entera (::after), pero el nombre
                   accesible sigue siendo el titulo, no todo el texto. -->
              <h2 class="titulo">
                <RouterLink :to="`/admin/configuracion/${t.clave}`" class="enlace">{{ t.titulo }}</RouterLink>
              </h2>
              <BaseBadge :variant="t.publica ? 'accent' : 'neutral'" sr-label="Visibilidad">
                {{ t.publica ? 'Publica' : 'Privada' }}
              </BaseBadge>
            </div>
            <p v-if="t.descripcion" class="descripcion">{{ t.descripcion }}</p>

            <dl v-if="t.conEtiquetas" class="valores">
              <div v-for="(l, i) in t.lineas" :key="i" class="valor">
                <dt>{{ l.etiqueta }}</dt>
                <dd :class="l.tipo">
                  <template v-if="l.tipo === 'color'">
                    <span class="muestra" :style="{ background: l.color }" aria-hidden="true" />
                    <code>{{ l.color }}</code>
                  </template>
                  <code v-else-if="l.tipo === 'json'">{{ l.texto }}</code>
                  <template v-else>{{ l.texto }}</template>
                </dd>
              </div>
            </dl>
            <p v-else v-for="(l, i) in t.lineas" :key="i" :class="['suelto', l.tipo]">
              <code v-if="l.tipo === 'json'">{{ l.texto }}</code>
              <template v-else-if="l.tipo === 'color'">
                <span class="muestra" :style="{ background: l.color }" aria-hidden="true" />
                <code>{{ l.color }}</code>
              </template>
              <template v-else>{{ l.texto }}</template>
            </p>

            <p class="pie">
              <code>{{ t.clave }}</code> · Actualizada el {{ t.actualizada }}
            </p>
          </li>
        </ul>
      </template>

      <p v-if="faltan > 0" class="referencia">Mostrando {{ tarjetas.length }} de {{ pagina?.page.totalElements }}.</p>
    </div>
  </section>
</template>

<style scoped>
h1 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-lg);
}
.intro {
  margin: 0 0 var(--space-4);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.esqueleto {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.fila-fantasma {
  height: 7rem;
  border-radius: var(--radius-lg);
  background: var(--color-surface-hover);
}
.referencia {
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.tarjetas {
  display: grid;
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}
.tarjeta {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.tarjeta:hover {
  background: var(--color-surface-hover);
}
/* El foco del enlace se ve en la tarjeta entera, que es lo que se pulsa. */
.tarjeta:has(.enlace:focus-visible) {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: var(--focus-ring);
}

.cabeza {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.titulo {
  margin: 0;
  font-size: var(--text-base);
}
.enlace {
  color: var(--color-text);
  text-decoration: none;
}
.enlace::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
}
.enlace:focus-visible {
  outline: none;
}
.descripcion {
  margin: calc(-1 * var(--space-2)) 0 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

/* Nombre a la izquierda, valor a la derecha; en angosto, uno sobre otro. */
.valores {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: var(--space-2) var(--space-4);
  margin: 0;
  font-size: var(--text-sm);
}
.valor {
  display: contents;
}
dt {
  color: var(--color-text-muted);
}
dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}
.suelto {
  margin: 0;
  font-size: var(--text-sm);
}
.vacio {
  color: var(--color-text-muted);
  font-style: italic;
}
.color {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.muestra {
  display: inline-block;
  flex: none;
  width: 1.25rem;
  height: 1.25rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  vertical-align: middle;
}

.pie {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}

@media (max-width: 30rem) {
  .valores {
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
  }
  dd + dt,
  .valor + .valor dt {
    margin-top: var(--space-2);
  }
}
</style>
