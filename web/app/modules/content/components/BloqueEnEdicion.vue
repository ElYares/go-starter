<script setup lang="ts">
// Un bloque de la pagina en el editor: su tipo, sus acciones y su formulario.
//
// Si el tipo no esta en el catalogo —un fork lo borro y una pagina vieja lo
// usa—, NO se descarta ni se revienta (CU-004 E3): se muestra como no
// reconocido, con su JSON, y se deja quitar. El servidor no deja guardar
// mientras siga ahi, y el aviso lo dice antes de que alguien lo intente.
import { computed } from 'vue'
import { BaseBadge, BaseButton } from '~/shared/ui'
import CampoDeEsquema from '~/shared/formularios/CampoDeEsquema.vue'
import type { Esquema } from '~/shared/formularios/esquema'
import { bloqueConError, type Bloque } from '../editor'

const props = withDefaults(
  defineProps<{
    bloque: Bloque
    indice: number
    total: number
    /** `undefined` si el tipo no esta en el catalogo. */
    esquema?: Esquema
    etiqueta?: string
    errores: Record<string, string>
    deshabilitado?: boolean
  }>(),
  { deshabilitado: false },
)

const emit = defineEmits<{
  'update:bloque': [bloque: Bloque]
  subir: []
  bajar: []
  quitar: []
}>()

const base = computed(() => `blocks[${props.indice}]`)
const conError = computed(() => bloqueConError(props.errores, props.indice))
const nombre = computed(() => props.etiqueta ?? props.bloque.type)

// Los errores del propio bloque, no de sus props: un id repetido o un tipo que
// el servidor no reconoce.
const erroresDelBloque = computed(() =>
  [`${base.value}.id`, `${base.value}.type`, `${base.value}.props`, base.value]
    .map((c) => props.errores[c])
    .filter(Boolean),
)
</script>

<template>
  <article
    class="bloque"
    :class="{ 'con-error': conError, desconocido: !esquema }"
    :data-bloque="bloque.id"
    :aria-invalid="conError || undefined"
  >
    <header class="cabecera">
      <div class="titulo">
        <span class="numero">{{ indice + 1 }}</span>
        <strong>{{ esquema ? nombre : 'Bloque no reconocido' }}</strong>
        <BaseBadge v-if="!esquema" variant="danger" sr-label="Tipo">{{ bloque.type }}</BaseBadge>
        <BaseBadge v-if="conError" variant="danger" sr-label="Estado">Con errores</BaseBadge>
      </div>
      <div v-if="!deshabilitado" class="acciones">
        <BaseButton variant="ghost" size="sm" :disabled="indice === 0" :aria-label="`Subir bloque ${indice + 1}`" @click="emit('subir')">
          Subir
        </BaseButton>
        <BaseButton
          variant="ghost"
          size="sm"
          :disabled="indice === total - 1"
          :aria-label="`Bajar bloque ${indice + 1}`"
          @click="emit('bajar')"
        >
          Bajar
        </BaseButton>
        <BaseButton variant="danger" size="sm" :aria-label="`Quitar bloque ${indice + 1}`" @click="emit('quitar')">
          Quitar
        </BaseButton>
      </div>
    </header>

    <p v-for="mensaje in erroresDelBloque" :key="mensaje" class="error" role="alert">{{ mensaje }}</p>

    <div v-if="!esquema" class="sin-esquema">
      <p class="pista">
        El tipo <code>{{ bloque.type }}</code> ya no existe en el catalogo de este sitio. Se conserva tal cual, pero la
        pagina no se puede guardar mientras este bloque siga aqui: quitalo para guardar.
      </p>
      <pre>{{ JSON.stringify(bloque.props, null, 2) }}</pre>
    </div>

    <CampoDeEsquema
      v-else
      :esquema="esquema"
      :etiqueta="nombre"
      :ruta="`${base}.props`"
      :errores="errores"
      requerido
      raiz
      :deshabilitado="deshabilitado"
      :model-value="bloque.props"
      @update:model-value="emit('update:bloque', { ...bloque, props: ($event ?? {}) as Bloque['props'] })"
    />
  </article>
</template>

<style scoped>
.bloque {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}
.con-error,
.desconocido {
  border-color: var(--color-danger);
}
.cabecera {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
  justify-content: space-between;
}
.titulo,
.acciones {
  display: flex;
  gap: var(--space-2);
  align-items: center;
}
.numero {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.error {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-danger);
}
.pista {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
pre,
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
pre {
  margin: 0;
  padding: var(--space-2);
  overflow-x: auto;
  background: var(--color-surface-hover);
  border-radius: var(--radius-sm);
}
</style>
