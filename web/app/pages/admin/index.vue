<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  BaseBadge,
  BaseButton,
  BaseDialog,
  BaseEmptyState,
  BaseField,
  BaseInput,
  BaseSelect,
  BaseTable,
  BaseToast,
  type ColumnaTabla,
} from '~/shared/ui'

// Esta ruta es SPA por routeRules: no se renderiza en servidor.
// El guard de sesion llega en CU-003; hoy no hay nada que proteger.
useHead({ title: 'Dashboard · go-starter' })

// Un muestrario, no una pantalla de producto: es lo que hace que los primitivos
// se vean funcionando juntos antes de que exista el CRUD de la fase 3, y lo que
// obliga al build a resolverlos de verdad. Un fork lo borra entero el primer
// dia, y por eso todo su estado vive aqui y no toca nada mas.
type Pagina = { id: string; titulo: string; estado: string }

const ESTADOS = [
  { value: 'borrador', label: 'Borrador' },
  { value: 'publicada', label: 'Publicada' },
]

const paginas = ref<Pagina[]>([{ id: 'p1', titulo: 'Portada', estado: 'publicada' }])

const columnas: ColumnaTabla[] = [
  { key: 'titulo', label: 'Titulo' },
  { key: 'estado', label: 'Estado' },
  { key: 'acciones', label: '', align: 'end' },
]

const dialogoAbierto = ref(false)
const avisoAbierto = ref(false)
const titulo = ref('')
const estado = ref('borrador')

const error = computed(() => (titulo.value.trim() === '' ? 'El titulo no puede ir vacio' : ''))

function crear() {
  if (error.value) return
  paginas.value.push({
    id: crypto.randomUUID(),
    titulo: titulo.value.trim(),
    estado: estado.value,
  })
  titulo.value = ''
  dialogoAbierto.value = false
  avisoAbierto.value = true
}
</script>

<template>
  <main class="admin">
    <header class="encabezado">
      <div>
        <BaseBadge variant="accent">fase 1</BaseBadge>
        <h1>Dashboard</h1>
      </div>
      <BaseButton @click="dialogoAbierto = true">Nueva pagina</BaseButton>
    </header>

    <p class="hint">
      Sin SSR a proposito: nadie indexa un panel privado, y renderizarlo en
      servidor obligaria a reenviar cookies de sesion en cada navegacion.
      Lo de abajo es un muestrario de <code>shared/ui/</code>: un fork lo borra.
    </p>

    <BaseTable :columns="columnas" :rows="paginas" caption="Paginas de la landing">
      <template #celda-estado="{ valor }">
        <BaseBadge :variant="valor === 'publicada' ? 'ok' : 'neutral'" sr-label="Estado">
          {{ valor === 'publicada' ? 'Publicada' : 'Borrador' }}
        </BaseBadge>
      </template>
      <template #celda-acciones="{ fila }">
        <BaseButton
          variant="ghost"
          size="sm"
          @click="paginas = paginas.filter((p) => p.id !== fila.id)"
        >
          Quitar
        </BaseButton>
      </template>
      <template #vacio>
        <BaseEmptyState
          title="Todavia no hay paginas"
          description="Crea la primera y apareceria publicada en la landing."
        >
          <template #accion>
            <BaseButton @click="dialogoAbierto = true">Nueva pagina</BaseButton>
          </template>
        </BaseEmptyState>
      </template>
    </BaseTable>

    <BaseDialog
      v-model:open="dialogoAbierto"
      title="Nueva pagina"
      description="Se crea en borrador salvo que elijas otra cosa."
    >
      <div class="formulario">
        <BaseField label="Titulo" required :error="titulo === '' ? undefined : error" v-slot="campo">
          <BaseInput
            v-model="titulo"
            :id="campo.id"
            :described-by="campo.describedBy"
            :invalid="campo.invalid"
            placeholder="Sobre nosotros"
          />
        </BaseField>

        <BaseField label="Estado" v-slot="campo">
          <BaseSelect v-model="estado" :id="campo.id" :options="ESTADOS" />
        </BaseField>
      </div>

      <template #acciones>
        <BaseButton variant="secondary" @click="dialogoAbierto = false">Cancelar</BaseButton>
        <BaseButton :disabled="titulo.trim() === ''" @click="crear">Crear</BaseButton>
      </template>
    </BaseDialog>

    <BaseToast
      v-model:open="avisoAbierto"
      variant="ok"
      title="Pagina creada"
      description="En la fase 3 esto ira a la base y aparecera en la landing."
    />

    <NuxtLink to="/">Volver a la landing</NuxtLink>
  </main>
</template>

<style scoped>
.admin {
  max-width: 46rem;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
}
.encabezado {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: var(--space-4);
}
h1 {
  margin: var(--space-2) 0 0;
  font-size: var(--text-display);
}
.hint {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.formulario {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
.admin > a {
  display: inline-block;
  margin-top: var(--space-6);
}
</style>
