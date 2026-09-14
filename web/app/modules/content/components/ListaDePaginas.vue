<script setup lang="ts">
// Las paginas de la landing, en los estados de docs/07-frontend.md mas "sin
// permiso", y el alta de una pagina nueva. Sin Nuxt, como ListaDeSettings.
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseBadge, BaseButton, BaseDialog, BaseEmptyState, BaseField, BaseInput, BaseTable, type ColumnaTabla } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { erroresPorCampo, sugerirSlug } from '../editor'

const props = defineProps<{
  cargar: () => Promise<Schemas['PaginasPage']>
  crear: (nueva: Schemas['PaginaNueva']) => Promise<Schemas['Pagina']>
  puedeEscribir: boolean
}>()

const emit = defineEmits<{ creada: [pagina: Schemas['Pagina']] }>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const pagina = ref<Schemas['PaginasPage'] | null>(null)
const fallo = ref<ApiError | null>(null)

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    pagina.value = await props.cargar()
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
  { key: 'title', label: 'Pagina' },
  { key: 'slug', label: 'Direccion' },
  { key: 'publishedNumber', label: 'Publicacion' },
  { key: 'updatedAt', label: 'Actualizada' },
]
const filas = computed(() => (pagina.value?.content ?? []) as unknown as Array<Record<string, unknown>>)

const direccion = (slug: unknown) => (slug === 'inicio' ? '/' : `/${String(slug)}`)

// --- Alta

const dialogo = ref(false)
const nueva = ref({ title: '', slug: '' })
// El slug se sugiere desde el titulo hasta que la persona lo toca: despues, lo
// que escribio manda.
const slugTocado = ref(false)
const creando = ref(false)
const erroresAlta = ref<Record<string, string>>({})
const falloAlta = ref<string | null>(null)

watch(
  () => nueva.value.title,
  (titulo) => {
    if (!slugTocado.value) nueva.value.slug = sugerirSlug(titulo)
  },
)

function abrirAlta() {
  nueva.value = { title: '', slug: '' }
  slugTocado.value = false
  erroresAlta.value = {}
  falloAlta.value = null
  dialogo.value = true
}

async function onCrear() {
  creando.value = true
  erroresAlta.value = {}
  falloAlta.value = null
  try {
    const creada = await props.crear({ title: nueva.value.title, slug: nueva.value.slug, blocks: [] })
    dialogo.value = false
    emit('creada', creada)
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      erroresAlta.value = erroresPorCampo(causa)
    } else if (causa.status === 409) {
      erroresAlta.value = { slug: 'Ya hay una pagina con esta direccion. Elige otra.' }
    } else if (causa.status === 403) {
      falloAlta.value = 'No tienes permiso para crear paginas.'
    } else {
      falloAlta.value = causa.unavailable ? 'El servidor no responde. Vuelve a intentarlo.' : causa.message
    }
  } finally {
    creando.value = false
  }
}
</script>

<template>
  <section class="paginas" :aria-busy="estado === 'cargando' || undefined">
    <header class="cabecera">
      <h1>Paginas</h1>
      <BaseButton v-if="puedeEscribir && estado === 'listo'" @click="abrirAlta">Nueva pagina</BaseButton>
    </header>

    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando las paginas">
      <div v-for="n in 4" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          sinPermiso
            ? 'No tienes permiso para ver las paginas'
            : fallo?.unavailable
              ? 'El servidor no responde'
              : 'No se pudieron cargar las paginas'
        "
        :description="
          sinPermiso
            ? 'Pidele a quien administra el sitio el permiso content.page.read.'
            : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="!sinPermiso" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="referencia">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else data-estado="listo">
      <BaseTable :columns="columnas" :rows="filas" row-key="id" caption="Paginas de la landing">
        <template #celda-title="{ fila }">
          <RouterLink :to="`/admin/paginas/${fila.id}`" class="enlace">{{ fila.title }}</RouterLink>
        </template>
        <template #celda-slug="{ valor }">
          <code>{{ direccion(valor) }}</code>
        </template>
        <template #celda-publishedNumber="{ fila }">
          <BaseBadge v-if="fila.publishedNumber === null" variant="neutral" sr-label="Publicacion">Sin publicar</BaseBadge>
          <BaseBadge v-else-if="fila.publishedNumber === fila.draftNumber" variant="ok" sr-label="Publicacion">
            Publicada
          </BaseBadge>
          <BaseBadge v-else variant="accent" sr-label="Publicacion">Cambios sin publicar</BaseBadge>
        </template>
        <template #celda-updatedAt="{ valor }">
          {{ String(valor).slice(0, 10) }}
        </template>
        <template #vacio>
          <BaseEmptyState
            title="Todavia no hay paginas"
            :description="
              puedeEscribir
                ? 'Crea la primera: nace sin publicar, y la publicas cuando este lista.'
                : 'Cuando alguien con permiso de edicion cree una, aparecera aqui.'
            "
          >
            <template #accion>
              <BaseButton v-if="puedeEscribir" @click="abrirAlta">Crear la primera pagina</BaseButton>
              <BaseButton v-else variant="secondary" @click="traer">Volver a cargar</BaseButton>
            </template>
          </BaseEmptyState>
        </template>
      </BaseTable>
    </div>

    <BaseDialog v-model:open="dialogo" title="Nueva pagina" description="Nace vacia y sin publicar.">
      <form class="alta" @submit.prevent="onCrear">
        <BaseField label="Titulo" :error="erroresAlta.title" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput :id="id" v-model="nueva.title" :described-by="describedBy" :invalid="invalid" data-campo="title" />
          </template>
        </BaseField>
        <BaseField label="Direccion" :error="erroresAlta.slug" :hint="`La pagina vivira en /${nueva.slug}`" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput
              :id="id"
              v-model="nueva.slug"
              :described-by="describedBy"
              :invalid="invalid"
              data-campo="slug"
              @update:model-value="slugTocado = true"
            />
          </template>
        </BaseField>
        <p v-if="falloAlta" class="error" role="alert">{{ falloAlta }}</p>
      </form>
      <template #acciones>
        <BaseButton variant="secondary" @click="dialogo = false">Cancelar</BaseButton>
        <BaseButton :loading="creando" @click="onCrear">Crear pagina</BaseButton>
      </template>
    </BaseDialog>
  </section>
</template>

<style scoped>
.cabecera {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  justify-content: space-between;
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
.enlace {
  color: var(--color-accent-strong);
  font-weight: 600;
}
.alta {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.referencia {
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.error {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-danger);
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
</style>
