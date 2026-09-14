<script setup lang="ts">
import type { Schemas } from '~/shared/api/generated'
import { BloqueDesconocido, resolverBloque } from './registry'

// Recorre los bloques de una pagina y pinta cada uno con su componente. Lo usa
// la landing, y lo usara la vista previa del editor: una sola forma de pintar.
defineProps<{ bloques: Schemas['Bloque'][] }>()
</script>

<template>
  <template v-for="bloque in bloques" :key="bloque.id">
    <component
      :is="resolverBloque(bloque.type)!.component"
      v-if="resolverBloque(bloque.type)"
      :contenido="bloque.props"
    />
    <BloqueDesconocido v-else :tipo="bloque.type" :id="bloque.id" />
  </template>
</template>
