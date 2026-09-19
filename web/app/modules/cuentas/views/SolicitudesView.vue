<script setup lang="ts">
import ListaDeSolicitudes from '../components/ListaDeSolicitudes.vue'
import { asignarContrasena, listarSolicitudes } from '../api'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'

useHead({ title: 'Solicitudes · go-starter' })

const pedir = usePedirConSesion()
const cargar = () => pedir(() => listarSolicitudes(useApi()))
const asignar = (userId: string, password: string) => pedir(() => asignarContrasena(useApi(), userId, password))
</script>

<template>
  <ListaDeSolicitudes :cargar="cargar" :asignar="asignar" />
</template>
