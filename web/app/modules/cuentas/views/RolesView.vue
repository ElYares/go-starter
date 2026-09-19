<script setup lang="ts">
import ListaDeRoles from '../components/ListaDeRoles.vue'
import { listarRoles } from '../api'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'

useHead({ title: 'Roles · go-starter' })

const pedir = usePedirConSesion()
const cargar = () => pedir(() => listarRoles(useApi()))
</script>

<template>
  <ListaDeRoles :cargar="cargar" />
</template>
