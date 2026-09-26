<script setup lang="ts">
// Los roles y lo que concede cada uno (HU-018), de solo lectura. Los roles los
// siembran las migraciones y sus permisos la siembra del arranque; aqui solo se
// ven. Lo sensible —lo que reparte poder en vez de usarlo— va marcado, porque
// es lo que hay que mirar dos veces antes de darle ese rol a alguien.
//
// Se lee para quien administra, no para quien programa: primero la frase, por
// area, y con lo que le falta a cada rol a la vista. La clave queda al lado y
// en gris, porque es lo que nombran los mensajes de "no tienes permiso".
import { computed, onMounted, ref } from 'vue'
import { BaseBadge, BaseButton, BaseEmptyState } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { catalogo, enumerar, leer, type Alcance } from '../lectura-de-roles'

const props = defineProps<{ cargar: () => Promise<Schemas['RolesPage']> }>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const roles = ref<Schemas['Rol'][]>([])
const fallo = ref<ApiError | null>(null)

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    roles.value = (await props.cargar()).content
    estado.value = 'listo'
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

const todos = computed(() => catalogo(roles.value))
const lecturas = computed(() => roles.value.map((rol) => ({ rol, ...leer(rol, todos.value) })))
const haySensibles = computed(() => todos.value.some((p) => p.sensitive))

function resumen(alcance: Alcance): string {
  switch (alcance.tipo) {
    case 'nada':
      return 'Todavia no concede nada: quien solo tenga este rol entra al dashboard pero no ve ninguna seccion.'
    case 'todo':
      return 'Puede todo lo que hay en el dashboard.'
    case 'parte':
      return `Le faltan ${alcance.faltan} de ${alcance.total} permisos, de ${enumerar(alcance.areas)}. Van en gris abajo.`
  }
}

onMounted(traer)
</script>

<template>
  <section class="roles" :aria-busy="estado === 'cargando' || undefined">
    <header class="cabecera">
      <h1>Roles</h1>
      <p class="pista">Que puede hacer quien tiene cada rol. Un rol se asigna desde la ficha de la cuenta.</p>
      <p v-if="estado === 'listo' && haySensibles" class="leyenda" data-leyenda>
        <BaseBadge variant="danger">Da acceso</BaseBadge>
        <span>Con ese permiso se puede dar o quitar acceso a otras personas. Dalo solo a quien confies.</span>
      </p>
    </header>

    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando los roles">
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          fallo?.status === 403
            ? 'No tienes permiso para ver los roles'
            : fallo?.unavailable
              ? 'El servidor no responde'
              : 'No se pudieron cargar los roles'
        "
        :description="
          fallo?.status === 403
            ? 'Pidele a quien administra el sitio el permiso identity.role.read.'
            : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="fallo?.status !== 403" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista centrada">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else-if="roles.length === 0" data-estado="vacio">
      <BaseEmptyState
        title="No hay roles"
        description="Los siembra la migracion de identity. Si no estan, esa migracion no corrio: revisa el arranque del api."
      >
        <template #accion>
          <BaseButton variant="secondary" @click="traer">Volver a cargar</BaseButton>
        </template>
      </BaseEmptyState>
    </div>

    <div v-else class="lista" data-estado="listo">
      <article v-for="{ rol, alcance, grupos } in lecturas" :key="rol.key" class="rol" :data-rol="rol.key">
        <header>
          <h2>{{ rol.name }}</h2>
          <code class="clave">{{ rol.key }}</code>
        </header>
        <p class="alcance" :data-alcance="alcance.tipo">{{ resumen(alcance) }}</p>

        <section v-for="grupo in grupos" :key="grupo.area" class="area" :data-area="grupo.area">
          <h3>{{ grupo.area }}</h3>
          <ul>
            <li
              v-for="{ permiso, concedido } in grupo.filas"
              :key="permiso.key"
              :class="{ falta: !concedido }"
              :data-permiso="permiso.key"
              :data-concedido="concedido"
            >
              <span class="marca" aria-hidden="true">{{ concedido ? '✓' : '–' }}</span>
              <span class="texto">
                <span class="solo-lector">{{ concedido ? 'Puede' : 'No puede' }}: </span>
                {{ permiso.description }}
                <BaseBadge v-if="permiso.sensitive" variant="danger">Da acceso</BaseBadge>
              </span>
              <code class="clave">{{ permiso.key }}</code>
            </li>
          </ul>
        </section>
      </article>
    </div>
  </section>
</template>

<style scoped>
h1 {
  margin: 0;
  font-size: var(--text-lg);
}
h2 {
  margin: 0;
  font-size: var(--text-base);
}
.cabecera {
  margin-bottom: var(--space-4);
}
.esqueleto,
.lista {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.fila-fantasma {
  height: calc(var(--space-8) * 2);
  border-radius: var(--radius-sm);
  background: var(--color-surface-hover);
}
.rol {
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.rol header {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: baseline;
}
.alcance {
  margin: 0;
  font-size: var(--text-sm);
}
.leyenda {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
  margin: var(--space-3) 0 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
h3 {
  margin: 0 0 var(--space-1);
  font-size: var(--text-sm);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--color-text-muted);
}
ul {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}
li {
  display: grid;
  grid-template-columns: 1.25em minmax(0, 1fr) auto;
  gap: var(--space-2);
  align-items: baseline;
  font-size: var(--text-sm);
}
.marca {
  color: var(--color-ok);
  font-weight: 600;
}
.falta,
.falta .marca {
  color: var(--color-text-muted);
}
.texto {
  overflow-wrap: anywhere;
}
.texto :deep(.pildora) {
  margin-left: var(--space-1);
}
.clave {
  color: var(--color-text-muted);
}
/* En angosto la clave de cada permiso se va: bajo el texto duplicaba el largo
   de la pagina, y en el telefono nadie la esta buscando. La del rol se queda. */
@media (max-width: 40rem) {
  li {
    grid-template-columns: 1.25em minmax(0, 1fr);
  }
  li .clave {
    display: none;
  }
}
.solo-lector {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
.pista {
  margin: 0;
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
