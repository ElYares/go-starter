<script setup lang="ts">
// Los roles y lo que concede cada uno (HU-018), de solo lectura. Los roles los
// siembran las migraciones y sus permisos la siembra del arranque; aqui solo se
// ven. Lo sensible —lo que reparte poder en vez de usarlo— va marcado, porque
// es lo que hay que mirar dos veces antes de darle ese rol a alguien.
import { onMounted, ref } from 'vue'
import { BaseBadge, BaseButton, BaseEmptyState } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'

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

onMounted(traer)
</script>

<template>
  <section class="roles" :aria-busy="estado === 'cargando' || undefined">
    <header class="cabecera">
      <h1>Roles</h1>
      <p class="pista">Lo que puede hacer cada rol. Se reparten desde la ficha de cada cuenta.</p>
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
      <article v-for="rol in roles" :key="rol.key" class="rol" :data-rol="rol.key">
        <header>
          <h2>{{ rol.name }}</h2>
          <code>{{ rol.key }}</code>
        </header>
        <p v-if="rol.permissions.length === 0" class="pista">No concede ningun permiso todavia.</p>
        <ul v-else>
          <li v-for="p in rol.permissions" :key="p.key">
            <code>{{ p.key }}</code>
            <span>{{ p.description }}</span>
            <BaseBadge v-if="p.sensitive" variant="danger">Reparte poder</BaseBadge>
          </li>
        </ul>
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
}
.rol header {
  display: flex;
  gap: var(--space-2);
  align-items: baseline;
}
ul {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: var(--space-2) 0 0;
  padding: 0;
  list-style: none;
}
li {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
  font-size: var(--text-sm);
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
