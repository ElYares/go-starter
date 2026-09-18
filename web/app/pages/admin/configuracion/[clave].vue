<script setup lang="ts">
import EditarSettingView from '~/modules/settings/views/EditarSettingView.vue'

// La clave por ruta: pasar de una clave a otra monta el editor de nuevo, en vez
// de reutilizarlo con el valor de la anterior.
definePageMeta({ key: (route) => String(route.params.clave) })

const route = useRoute()
</script>

<template>
  <EditarSettingView :clave="String(route.params.clave)" />
</template>
