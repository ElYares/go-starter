<script setup lang="ts">
// La configuracion del sitio, en sus estados de docs/07-frontend.md mas el de
// "sin permiso". Sin nada de Nuxt: recibe `cargar` y no sabe de donde salen los
// datos, asi se prueba cada estado con una promesa a mano.
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseBadge, BaseButton, BaseEmptyState, BaseTable, type ColumnaTabla } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'

const props = defineProps<{
  cargar: () => Promise<Schemas['SettingsPage']>
}>()

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

const columnas: ColumnaTabla[] = [
  { key: 'key', label: 'Clave' },
  { key: 'value', label: 'Valor' },
  { key: 'isPublic', label: 'Visibilidad' },
  { key: 'updatedAt', label: 'Actualizada' },
]

const filas = computed(() => (pagina.value?.content ?? []) as unknown as Array<Record<string, unknown>>)
const faltan = computed(() => {
  const p = pagina.value
  return p ? p.page.totalElements - p.content.length : 0
})

// El valor es JSON libre. Se muestra compacto y recortado: una clave con un
// objeto de cien campos no puede ensanchar la tabla hasta sacarla de la pantalla.
function resumir(valor: unknown): string {
  const texto = JSON.stringify(valor)
  return texto.length > 80 ? `${texto.slice(0, 79)}…` : texto
}
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
      <BaseTable :columns="columnas" :rows="filas" row-key="key" caption="Claves de configuracion del sitio">
        <!-- Cada clave abre su editor (CU-006). -->
        <template #celda-key="{ valor }">
          <RouterLink :to="`/admin/configuracion/${valor}`" class="enlace">{{ valor }}</RouterLink>
        </template>
        <template #celda-value="{ valor }">
          <code>{{ resumir(valor) }}</code>
        </template>
        <template #celda-isPublic="{ valor }">
          <BaseBadge :variant="valor ? 'accent' : 'neutral'" sr-label="Visibilidad">
            {{ valor ? 'Publica' : 'Privada' }}
          </BaseBadge>
        </template>
        <template #celda-updatedAt="{ valor }">
          {{ String(valor).slice(0, 10) }}
        </template>
        <template #vacio>
          <BaseEmptyState
            title="Todavia no hay configuracion"
            description="Las claves se crean con POST /api/v1/settings, con el permiso settings.write."
          >
            <template #accion>
              <BaseButton variant="secondary" @click="traer">Volver a cargar</BaseButton>
            </template>
          </BaseEmptyState>
        </template>
      </BaseTable>
      <p v-if="faltan > 0" class="referencia">Mostrando {{ filas.length }} de {{ pagina?.page.totalElements }}.</p>
    </div>
  </section>
</template>

<style scoped>
h1 {
  margin: 0 0 var(--space-4);
  font-size: var(--text-lg);
}
.esqueleto {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.fila-fantasma {
  height: var(--space-8);
  border-radius: var(--radius-sm);
  background: var(--color-surface-hover);
}
.referencia {
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.enlace {
  color: var(--color-accent-strong);
  font-family: var(--font-mono);
  font-weight: 600;
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
</style>
