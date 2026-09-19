<script setup lang="ts">
// Las solicitudes de contrasena pendientes (HU-019), en los estados de
// docs/07-frontend.md mas "sin permiso". Cada una se atiende asignando una
// contrasena temporal, que se entrega por fuera: el starter no manda correos.
import { computed, onMounted, ref } from 'vue'
import { BaseButton, BaseEmptyState, BaseTable, type ColumnaTabla } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import DialogoDeContrasenaTemporal from './DialogoDeContrasenaTemporal.vue'

type Solicitud = Schemas['Solicitud']

const props = defineProps<{
  cargar: () => Promise<Schemas['SolicitudesPage']>
  asignar: (userId: string, password: string) => Promise<void>
}>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const solicitudes = ref<Solicitud[]>([])
const fallo = ref<ApiError | null>(null)

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    solicitudes.value = (await props.cargar()).content
    estado.value = 'listo'
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

onMounted(traer)

const sinPermiso = computed(() => fallo.value?.status === 403)

const columnas: ColumnaTabla[] = [
  { key: 'displayName', label: 'Nombre' },
  { key: 'email', label: 'Correo' },
  { key: 'createdAt', label: 'Pedida' },
  { key: 'acciones', label: 'Acciones' },
]
const filas = computed(() => solicitudes.value as unknown as Array<Record<string, unknown>>)

const elegida = ref<Solicitud | null>(null)
const dialogo = ref(false)

function atender(fila: Record<string, unknown>) {
  elegida.value = fila as unknown as Solicitud
  dialogo.value = true
}

// Atendida, sale de la lista: el api ya la dio por resuelta.
function alAsignar() {
  const id = elegida.value?.id
  solicitudes.value = solicitudes.value.filter((s) => s.id !== id)
}

const fecha = (iso: unknown) => String(iso).slice(0, 16).replace('T', ' ')
</script>

<template>
  <section :aria-busy="estado === 'cargando' || undefined">
    <header class="cabecera">
      <h1>Solicitudes de contrasena</h1>
      <p class="pista">Personas que olvidaron su contrasena y pidieron una temporal desde el login.</p>
    </header>

    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando las solicitudes">
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          sinPermiso
            ? 'No tienes permiso para ver las solicitudes'
            : fallo?.unavailable
              ? 'El servidor no responde'
              : 'No se pudieron cargar las solicitudes'
        "
        :description="
          sinPermiso
            ? 'Pidele a quien administra el sitio el permiso identity.user.password.'
            : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="!sinPermiso" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista centrada">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else data-estado="listo">
      <BaseTable :columns="columnas" :rows="filas" row-key="id" caption="Solicitudes pendientes">
        <template #celda-createdAt="{ valor }">{{ fecha(valor) }}</template>
        <template #celda-acciones="{ fila }">
          <BaseButton size="sm" :data-solicitud="fila.id" @click="atender(fila)">Asignar contrasena temporal</BaseButton>
        </template>
        <template #vacio>
          <BaseEmptyState
            title="No hay solicitudes pendientes"
            description="Cuando alguien pida una contrasena desde el login, aparecera aqui."
          >
            <template #accion>
              <BaseButton variant="secondary" @click="traer">Volver a cargar</BaseButton>
            </template>
          </BaseEmptyState>
        </template>
      </BaseTable>
    </div>

    <DialogoDeContrasenaTemporal
      v-if="elegida"
      v-model:open="dialogo"
      :cuenta="elegida"
      :asignar="(password) => asignar(elegida!.userId, password)"
      @asignada="alAsignar"
    />
  </section>
</template>

<style scoped>
.cabecera {
  margin-bottom: var(--space-4);
}
h1 {
  margin: 0;
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
.pista {
  margin: var(--space-1) 0 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.centrada {
  text-align: center;
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
</style>
