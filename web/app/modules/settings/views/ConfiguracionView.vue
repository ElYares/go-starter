<script setup lang="ts">
import ListaDeSettings from '../components/ListaDeSettings.vue'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

useHead({ title: 'Configuracion · go-starter' })

const pedir = usePedirConSesion()

// Primera pagina, con el tope del molde. `page.totalElements` dice si faltan.
const cargar = () => pedir(() => useApi().get<Schemas['SettingsPage']>('/settings?size=100'))
</script>

<template>
  <ListaDeSettings :cargar="cargar" />
</template>
