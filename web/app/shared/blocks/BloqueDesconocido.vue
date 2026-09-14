<script setup lang="ts">
// Lo que se pinta cuando un `type` no esta en el registro: nada.
//
// Un fork que borra un tipo de bloque no puede tumbar las paginas ya publicadas
// que lo usan (Decision 006). Se renderiza el resto de la pagina y queda el
// aviso: en SSR sale en el log del servidor de Nuxt, que es donde alguien lo
// va a buscar; en el navegador, en la consola.
const props = defineProps<{ tipo: string; id: string }>()

console.warn(`[bloques] tipo desconocido "${props.tipo}" en el bloque "${props.id}": no se renderiza`)
</script>

<template>
  <template v-if="false" />
</template>
