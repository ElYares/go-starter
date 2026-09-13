<script setup lang="ts">
import { ref } from 'vue'
import AdminShell from '../components/AdminShell.vue'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { cerrarSesion, RUTA_LOGIN } from '~/modules/auth/sesion'
import { useApi } from '~/shared/api/useApi'

// El marco de todo /admin. Es la ruta padre (pages/admin.vue), asi que no se
// vuelve a montar al pasar de una seccion a otra: solo cambia el <NuxtPage>.
//
// El perfil esta garantizado por middleware/sesion.global.ts, que no deja
// llegar aqui sin el. El v-if cubre el instante de "Cerrar sesion", en que el
// perfil ya es null y la navegacion al login todavia no termino.
const { perfil } = useSesion()
const saliendo = ref(false)

// El perfil se vacia ANTES de navegar: si quedara, el guard dejaria volver a
// /admin con el boton de atras sin preguntar a nadie.
async function salir() {
  saliendo.value = true
  await cerrarSesion(useApi())
  perfil.value = null
  await navigateTo(RUTA_LOGIN, { replace: true })
}
</script>

<template>
  <AdminShell v-if="perfil" :perfil="perfil" :saliendo="saliendo" @salir="salir">
    <NuxtPage />
  </AdminShell>
</template>
