<script setup lang="ts">
import PaginaView from '~/modules/landing/views/PaginaView.vue'

// Toda ruta que no es /admin ni /login es una pagina del CMS. La clave por ruta
// completa hace que pasar de /precios a /nosotros monte la vista de nuevo y
// pida la otra pagina, en vez de reutilizar la instancia con los datos viejos.
definePageMeta({ key: (route) => route.fullPath })

const route = useRoute()
</script>

<template>
  <PaginaView :segmentos="route.params.slug" />
</template>
