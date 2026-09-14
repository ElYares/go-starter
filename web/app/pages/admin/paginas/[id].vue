<script setup lang="ts">
import EditorView from '~/modules/content/views/EditorView.vue'

// La clave por id: pasar de una pagina a otra monta el editor de nuevo, en vez
// de reutilizarlo con el borrador de la anterior.
definePageMeta({ key: (route) => String(route.params.id) })

const route = useRoute()
</script>

<template>
  <EditorView :id="String(route.params.id)" />
</template>
