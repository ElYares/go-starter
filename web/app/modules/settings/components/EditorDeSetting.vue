<script setup lang="ts">
// El editor de una clave de configuracion (CU-006). Sin nada de Nuxt, como el
// editor de paginas: recibe las operaciones como funciones, asi se prueba cada
// camino —guardar, el 400, el 409, la subida— con promesas a mano.
//
// El formulario sale del JSON Schema de la clave (Decision 022). Las reglas que
// no se ven en el marcado son las del editor de paginas:
//
// - **Lo escrito no se pierde en un error.** Un 400 marca campos y un 409 avisa;
//   ninguno de los dos toca lo que hay en pantalla
// - **Un 409 no guarda encima.** Ofrece descartar y recargar
// - **La visibilidad no se toca**: el guardado no manda `isPublic`, y ausente la
//   conserva (Decision 025)
import { computed, onMounted, ref, watch } from 'vue'
import { BaseBadge, BaseButton, BaseEmptyState, BaseToast } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import CampoDeEsquema from '~/shared/formularios/CampoDeEsquema.vue'
import { erroresPorCampo, type Esquema } from '~/shared/formularios/esquema'
import { esquemas as esquemasDelApi } from '../esquemas'

type Setting = Schemas['Setting']

const props = withDefaults(
  defineProps<{
    clave: string
    cargar: () => Promise<Setting>
    guardar: (version: number, valor: unknown) => Promise<Setting>
    subirMedio?: (archivo: File) => Promise<Schemas['Medio']>
    puedeEscribir: boolean
    esquemas?: Record<string, Esquema>
  }>(),
  { esquemas: () => esquemasDelApi },
)

const emit = defineEmits<{
  /** Si hay cambios sin guardar, para que la vista avise antes de salir. */
  cambios: [pendientes: boolean]
}>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const fallo = ref<ApiError | null>(null)
const setting = ref<Setting | null>(null)
const valor = ref<unknown>()

const errores = ref<Record<string, string>>({})
const aviso = ref<{ titulo: string; detalle?: string; traceId?: string } | null>(null)
const conflicto = ref<{ guardadaEl?: string } | null>(null)
const guardando = ref(false)
const toast = ref({ abierto: false, titulo: '' })

const esquema = computed(() => props.esquemas[props.clave])
const titulo = computed(() => esquema.value?.title ?? props.clave)

// Copia por JSON y no con structuredClone: el valor llega envuelto en un proxy
// reactivo, y structuredClone lanza con un proxy (docs/07-frontend.md).
const copia = (v: unknown) => (v === undefined ? undefined : JSON.parse(JSON.stringify(v)))

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    const leido = await props.cargar()
    setting.value = leido
    valor.value = copia(leido.value)
    errores.value = {}
    aviso.value = null
    conflicto.value = null
    estado.value = 'listo'
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

onMounted(traer)

const pendientes = computed(() =>
  setting.value ? JSON.stringify(valor.value) !== JSON.stringify(setting.value.value) : false,
)
watch(pendientes, (v) => emit('cambios', v))

const cuantosErrores = computed(() => Object.keys(errores.value).length)

async function onGuardar() {
  // Un Enter en un campo envia el formulario: sin cambios no se guarda, porque
  // guardar lo mismo sube la version y le da un 409 a quien si estaba editando.
  if (!setting.value || !pendientes.value || guardando.value) return
  guardando.value = true
  errores.value = {}
  aviso.value = null
  conflicto.value = null
  try {
    const guardado = await props.guardar(setting.value.version, valor.value)
    setting.value = guardado
    valor.value = copia(guardado.value)
    toast.value = { abierto: true, titulo: `${titulo.value} guardada` }
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      errores.value = erroresPorCampo(causa)
      const n = cuantosErrores.value
      aviso.value = {
        titulo: n === 0 ? 'Hay datos que no se aceptan' : `Hay ${n} ${n === 1 ? 'campo' : 'campos'} por corregir`,
        detalle: 'No se guardo nada. Lo que escribiste sigue aqui.',
        traceId: causa.traceId,
      }
    } else if (causa.status === 409) {
      // Se lee la version actual solo para decir cuando se guardo; NO se aplica
      // sobre lo que la persona tiene en pantalla.
      conflicto.value = {}
      props
        .cargar()
        .then((actual) => {
          if (conflicto.value) conflicto.value = { guardadaEl: actual.updatedAt }
        })
        .catch(() => {})
    } else if (causa.status === 403) {
      aviso.value = { titulo: 'No tienes permiso para cambiar la configuracion', traceId: causa.traceId }
    } else if (causa.unavailable) {
      aviso.value = {
        titulo: 'El servidor no responde',
        detalle: 'Tus cambios siguen aqui. Vuelve a intentarlo en un momento.',
        traceId: causa.traceId,
      }
    } else {
      aviso.value = { titulo: 'No se pudo guardar', detalle: causa.message, traceId: causa.traceId }
    }
  } finally {
    guardando.value = false
  }
}

function descartar() {
  if (!setting.value) return
  valor.value = copia(setting.value.value)
  errores.value = {}
  aviso.value = null
}

const fecha = (iso?: string) => (iso ? iso.slice(0, 16).replace('T', ' ') : '')
</script>

<template>
  <section class="editor" :aria-busy="estado === 'cargando' || undefined">
    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando la configuracion">
      <div class="fila-fantasma alta" />
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          fallo?.status === 403
            ? 'No tienes permiso para ver la configuracion'
            : fallo?.status === 404
              ? 'Esta clave no existe'
              : fallo?.unavailable
                ? 'El servidor no responde'
                : 'No se pudo cargar la configuracion'
        "
        :description="
          fallo?.status === 403
            ? 'Pidele a quien administra el sitio el permiso settings.read.'
            : fallo?.status === 404
              ? 'Puede que la migracion que la siembra no haya corrido.'
              : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="fallo?.status !== 403 && fallo?.status !== 404" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista centrada">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else-if="setting" class="listo" data-estado="listo">
      <header class="cabecera">
        <div>
          <h1>{{ titulo }}</h1>
          <p class="pista">
            <code>{{ clave }}</code>
            <template v-if="esquema?.description"> · {{ esquema.description }}</template>
          </p>
        </div>
        <div class="insignias">
          <BaseBadge :variant="setting.isPublic ? 'ok' : 'neutral'" sr-label="Visibilidad">
            {{ setting.isPublic ? 'Publica' : 'Privada' }}
          </BaseBadge>
          <BaseBadge v-if="pendientes" variant="danger" sr-label="Cambios">Sin guardar</BaseBadge>
        </div>
      </header>

      <div v-if="puedeEscribir && esquema" class="barra" role="toolbar" aria-label="Acciones de la configuracion">
        <BaseButton :loading="guardando" :disabled="!pendientes" @click="onGuardar">Guardar</BaseButton>
        <BaseButton variant="ghost" :disabled="!pendientes || guardando" @click="descartar">
          Descartar cambios
        </BaseButton>
      </div>

      <p v-if="!puedeEscribir" class="pista" data-aviso="solo-lectura">
        Solo lectura: para editar hace falta el permiso settings.write.
      </p>

      <div v-if="conflicto" class="aviso" role="alert" data-aviso="conflicto">
        <strong>Alguien guardo esta configuracion mientras la editabas.</strong>
        <p>
          Se guardo otra version{{ conflicto.guardadaEl ? ` el ${fecha(conflicto.guardadaEl)}` : '' }}. No se guardo
          nada encima: tus cambios siguen en pantalla. Para seguir, descartalos y recarga la version actual.
        </p>
        <BaseButton variant="secondary" size="sm" @click="traer">Descartar mis cambios y recargar</BaseButton>
      </div>

      <div v-if="aviso" class="aviso" role="alert" data-aviso="error">
        <strong>{{ aviso.titulo }}</strong>
        <p v-if="aviso.detalle">{{ aviso.detalle }}</p>
        <p v-if="aviso.traceId" class="pista">Referencia: <code>{{ aviso.traceId }}</code></p>
      </div>

      <!-- Una clave que el navegador no conoce: la agrego un fork en el api y el
           build de web no la trae todavia. Se ve, y no se edita a ciegas. -->
      <div v-if="!esquema" class="grupo" data-aviso="sin-esquema">
        <p class="pista">
          Esta clave no tiene formulario en el dashboard. Se muestra como esta; editala desde la API.
        </p>
        <pre>{{ JSON.stringify(setting.value, null, 2) }}</pre>
      </div>

      <form v-else class="formulario" @submit.prevent="onGuardar">
        <CampoDeEsquema
          v-model="valor"
          :esquema="esquema"
          :etiqueta="titulo"
          ruta="value"
          :errores="errores"
          requerido
          :raiz="esquema.type === 'object'"
          :deshabilitado="!puedeEscribir"
          :subir-medio="puedeEscribir ? subirMedio : undefined"
        />
      </form>

      <BaseToast v-model:open="toast.abierto" :title="toast.titulo" variant="ok" />
    </div>
  </section>
</template>

<style scoped>
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
.fila-fantasma.alta {
  height: calc(var(--space-8) * 2);
}
.listo,
.formulario {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  max-width: 40rem;
}
.cabecera {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  align-items: flex-start;
  justify-content: space-between;
}
.insignias,
.barra {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}
.aviso {
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.aviso p {
  margin: var(--space-1) 0 var(--space-2);
}
.grupo {
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}
.pista {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.centrada {
  text-align: center;
}
pre {
  margin: var(--space-2) 0 0;
  padding: var(--space-2);
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  background: var(--color-surface-hover);
  border-radius: var(--radius-sm);
}
</style>
