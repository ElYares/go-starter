<script setup lang="ts">
import ListaDeCuentas from '../components/ListaDeCuentas.vue'
import { crearCuenta, listarCuentas } from '../api'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

useHead({ title: 'Usuarios · go-starter' })

const pedir = usePedirConSesion()
const { perfil } = useSesion()

// Mostrar el boton de crear es conveniencia: si falta el permiso, el servidor
// responde 403 y el dialogo lo dice.
const puedeEscribir = computed(() => perfil.value?.permissions.includes('identity.user.write') ?? false)

const cargar = () => pedir(() => listarCuentas(useApi()))
const crear = (nueva: Schemas['CuentaNueva']) => pedir(() => crearCuenta(useApi(), nueva))

// La cuenta nace sin roles: se abre su ficha, que es donde se le dan.
const abrir = (c: Schemas['Cuenta']) => navigateTo(`/admin/usuarios/${c.id}`)
</script>

<template>
  <ListaDeCuentas :cargar="cargar" :crear="crear" :puede-escribir="puedeEscribir" @creada="abrir" />
</template>
