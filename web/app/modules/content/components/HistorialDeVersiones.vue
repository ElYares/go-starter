<script setup lang="ts">
// El historial de una pagina. Revertir es publicar una version anterior
// (CU-004 A2): no se borra ninguna, y por eso aqui no hay boton de borrar.
import { onMounted, ref, watch } from 'vue'
import { BaseBadge, BaseButton, BaseEmptyState } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'

const props = withDefaults(
  defineProps<{
    cargar: () => Promise<Schemas['VersionesPage']>
    puedePublicar: boolean
    /** Sube cada vez que algo cambio el historial fuera de aqui: un guardado. */
    recarga?: number
    /** La version que se esta publicando, si hay una. */
    publicando?: string | null
  }>(),
  { recarga: 0, publicando: null },
)

const emit = defineEmits<{ publicar: [versionId: string, numero: number] }>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const versiones = ref<Schemas['VersionResumen'][]>([])
const fallo = ref<ApiError | null>(null)

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    versiones.value = (await props.cargar()).content
    estado.value = 'listo'
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

onMounted(traer)
watch(() => props.recarga, traer)

const fecha = (iso: string) => iso.slice(0, 16).replace('T', ' ')
</script>

<template>
  <section class="historial" :aria-busy="estado === 'cargando' || undefined">
    <h2>Historial</h2>

    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando el historial">
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="fallo?.unavailable ? 'El servidor no responde' : 'No se pudo cargar el historial'"
        description="Vuelve a intentarlo. Si se repite, comparte la referencia de abajo."
      >
        <template #accion>
          <BaseButton variant="secondary" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <ol v-else class="versiones" data-estado="listo">
      <li v-for="v in versiones" :key="v.id" class="version" :data-version="v.number">
        <div class="datos">
          <strong>Version {{ v.number }}</strong>
          <BaseBadge v-if="v.published" variant="ok" sr-label="Estado">Publicada</BaseBadge>
          <span class="pista">{{ fecha(v.createdAt) }}</span>
          <span v-if="v.note" class="nota">{{ v.note }}</span>
        </div>
        <BaseButton
          v-if="puedePublicar && !v.published"
          variant="secondary"
          size="sm"
          :loading="publicando === v.id"
          :disabled="publicando !== null && publicando !== v.id"
          @click="emit('publicar', v.id, v.number)"
        >
          Publicar esta version
        </BaseButton>
      </li>
    </ol>
  </section>
</template>

<style scoped>
h2 {
  margin: 0 0 var(--space-3);
  font-size: var(--text-lg);
}
.esqueleto,
.versiones {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.fila-fantasma {
  height: var(--space-8);
  border-radius: var(--radius-sm);
  background: var(--color-surface-hover);
}
.version {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}
.datos {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}
.pista {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.nota {
  font-size: var(--text-sm);
}
code {
  font-family: var(--font-mono);
}
</style>
