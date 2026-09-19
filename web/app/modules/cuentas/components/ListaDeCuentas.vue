<script setup lang="ts">
// Las cuentas del dashboard (HU-018), en los estados de docs/07-frontend.md mas
// "sin permiso", y el alta de una cuenta nueva. Sin Nuxt, como ListaDePaginas.
//
// El alta no pide roles, igual que el api: repartirlos es otro permiso. Despues
// de crearla se abre su ficha, que es donde se le dan.
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { BaseBadge, BaseButton, BaseDialog, BaseEmptyState, BaseField, BaseInput, BaseTable, type ColumnaTabla } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import CampoDeContrasena from '~/shared/formularios/CampoDeContrasena.vue'
import { erroresPorCampo } from '~/shared/formularios/esquema'

const props = defineProps<{
  cargar: () => Promise<Schemas['CuentasPage']>
  crear: (nueva: Schemas['CuentaNueva']) => Promise<Schemas['Cuenta']>
  puedeEscribir: boolean
}>()

const emit = defineEmits<{ creada: [cuenta: Schemas['Cuenta']] }>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const pagina = ref<Schemas['CuentasPage'] | null>(null)
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
  { key: 'displayName', label: 'Nombre' },
  { key: 'email', label: 'Correo' },
  { key: 'roles', label: 'Roles' },
  { key: 'enabled', label: 'Acceso' },
]
const filas = computed(() => (pagina.value?.content ?? []) as unknown as Array<Record<string, unknown>>)

// --- Alta

const dialogo = ref(false)
const nueva = ref({ email: '', displayName: '', password: '' })
const creando = ref(false)
const erroresAlta = ref<Record<string, string>>({})
const falloAlta = ref<string | null>(null)

function abrirAlta() {
  nueva.value = { email: '', displayName: '', password: '' }
  erroresAlta.value = {}
  falloAlta.value = null
  dialogo.value = true
}

async function onCrear() {
  creando.value = true
  erroresAlta.value = {}
  falloAlta.value = null
  try {
    const creada = await props.crear({ ...nueva.value })
    dialogo.value = false
    emit('creada', creada)
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      erroresAlta.value = erroresPorCampo(causa)
    } else if (causa.status === 409) {
      // Al crear, el unico conflicto posible es el correo: no hay version.
      erroresAlta.value = { email: 'Ya hay una cuenta con este correo.' }
    } else if (causa.status === 403) {
      falloAlta.value = 'No tienes permiso para crear cuentas.'
    } else {
      falloAlta.value = causa.unavailable ? 'El servidor no responde. Vuelve a intentarlo.' : causa.message
    }
  } finally {
    creando.value = false
  }
}
</script>

<template>
  <section class="cuentas" :aria-busy="estado === 'cargando' || undefined">
    <header class="cabecera">
      <h1>Usuarios</h1>
      <BaseButton v-if="puedeEscribir && estado === 'listo'" @click="abrirAlta">Nueva cuenta</BaseButton>
    </header>

    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando las cuentas">
      <div v-for="n in 4" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          sinPermiso
            ? 'No tienes permiso para ver las cuentas'
            : fallo?.unavailable
              ? 'El servidor no responde'
              : 'No se pudieron cargar las cuentas'
        "
        :description="
          sinPermiso
            ? 'Pidele a quien administra el sitio el permiso identity.user.read.'
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
      <BaseTable :columns="columnas" :rows="filas" row-key="id" caption="Cuentas del dashboard">
        <template #celda-displayName="{ fila }">
          <RouterLink :to="`/admin/usuarios/${fila.id}`" class="enlace">{{ fila.displayName }}</RouterLink>
        </template>
        <template #celda-roles="{ valor }">
          <span class="roles">
            <BaseBadge v-for="rol in valor as string[]" :key="rol" variant="neutral">{{ rol }}</BaseBadge>
            <span v-if="(valor as string[]).length === 0" class="pista">Sin roles</span>
          </span>
        </template>
        <template #celda-enabled="{ valor }">
          <BaseBadge :variant="valor ? 'ok' : 'danger'" sr-label="Acceso">
            {{ valor ? 'Habilitada' : 'Deshabilitada' }}
          </BaseBadge>
        </template>
        <template #vacio>
          <!-- Casi imposible —quien ve esta lista tiene una cuenta—, pero un
               filtro futuro puede dejarla vacia, y el estado tiene que existir. -->
          <BaseEmptyState
            title="Todavia no hay cuentas"
            :description="
              puedeEscribir
                ? 'Crea la primera. Nace sin roles: se los das desde su ficha.'
                : 'Cuando alguien con permiso cree una, aparecera aqui.'
            "
          >
            <template #accion>
              <BaseButton v-if="puedeEscribir" @click="abrirAlta">Crear la primera cuenta</BaseButton>
              <BaseButton v-else variant="secondary" @click="traer">Volver a cargar</BaseButton>
            </template>
          </BaseEmptyState>
        </template>
      </BaseTable>
    </div>

    <BaseDialog v-model:open="dialogo" title="Nueva cuenta" description="Nace habilitada y sin roles.">
      <form class="alta" @submit.prevent="onCrear">
        <BaseField label="Nombre" :error="erroresAlta.displayName" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput :id="id" v-model="nueva.displayName" :described-by="describedBy" :invalid="invalid" data-campo="displayName" />
          </template>
        </BaseField>
        <BaseField label="Correo" :error="erroresAlta.email" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput :id="id" v-model="nueva.email" type="email" :described-by="describedBy" :invalid="invalid" data-campo="email" />
          </template>
        </BaseField>
        <BaseField
          label="Contrasena inicial"
          :error="erroresAlta.password"
          hint="Minimo 12 caracteres. Compartela por un canal seguro."
          required
        >
          <template #default="{ id, describedBy, invalid }">
            <CampoDeContrasena
              :id="id"
              v-model="nueva.password"
              :described-by="describedBy"
              :invalid="invalid"
              generador
              data-campo="password"
            />
          </template>
        </BaseField>
        <p v-if="falloAlta" class="error" role="alert">{{ falloAlta }}</p>
      </form>
      <template #acciones>
        <BaseButton variant="secondary" @click="dialogo = false">Cancelar</BaseButton>
        <BaseButton :loading="creando" @click="onCrear">Crear cuenta</BaseButton>
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
.roles {
  display: inline-flex;
  flex-wrap: wrap;
  gap: var(--space-1);
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
.pista {
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
