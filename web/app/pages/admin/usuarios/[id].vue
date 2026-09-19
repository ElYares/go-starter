<script setup lang="ts">
import CuentaView from '~/modules/cuentas/views/CuentaView.vue'

// La clave por id: pasar de una cuenta a otra monta la ficha de nuevo, en vez
// de reutilizarla con los datos de la anterior.
definePageMeta({ key: (route) => String(route.params.id) })

const route = useRoute()
</script>

<template>
  <CuentaView :id="String(route.params.id)" />
</template>
