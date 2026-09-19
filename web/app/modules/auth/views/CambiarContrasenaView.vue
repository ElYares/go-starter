<script setup lang="ts">
import CambiarContrasena from '../components/CambiarContrasena.vue'
import { cambiarContrasena } from '../contrasena'
import { useSesion } from '../composables/useSesion'
import { usePedirConSesion } from '../composables/usePedirConSesion'
import { pedirPerfil } from '../sesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

useHead({ title: 'Mi contrasena · go-starter' })

const pedir = usePedirConSesion()
const { perfil } = useSesion()
const temporal = computed(() => perfil.value?.mustChangePassword ?? false)

const cambiar = (cuerpo: Schemas['CambioDeContrasena']) => pedir(() => cambiarContrasena(useApi(), cuerpo))

// Se vuelve a pedir `me` antes de salir: con la temporal, el perfil guardado
// no tiene permisos y el guard volveria a mandar aqui.
async function alCambiar() {
  perfil.value = await pedir(() => pedirPerfil(useApi()))
  await navigateTo('/admin')
}
</script>

<template>
  <CambiarContrasena :cambiar="cambiar" :temporal="temporal" @cambiada="alCambiar" />
</template>
