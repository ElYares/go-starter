<script setup lang="ts">
import ListaDePaginas from '../components/ListaDePaginas.vue'
import { crearPagina, listarPaginas } from '../api'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

useHead({ title: 'Paginas · go-starter' })

const pedir = usePedirConSesion()
const { perfil } = useSesion()

// Mostrar el boton de crear es conveniencia: si falta el permiso, el servidor
// responde 403 y el dialogo lo dice.
const puedeEscribir = computed(() => perfil.value?.permissions.includes('content.page.write') ?? false)

const cargar = () => pedir(() => listarPaginas(useApi()))
const crear = (nueva: Schemas['PaginaNueva']) => pedir(() => crearPagina(useApi(), nueva))

const abrir = (p: Schemas['Pagina']) => navigateTo(`/admin/paginas/${p.id}`)
</script>

<template>
  <ListaDePaginas :cargar="cargar" :crear="crear" :puede-escribir="puedeEscribir" @creada="abrir" />
</template>
