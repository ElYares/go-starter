<script setup lang="ts">
// El marco de toda pagina de la landing: cabecera, contenido y pie. Lo usan la
// vista de pagina y la pantalla de error de la landing, para que un 404 tambien
// tenga por donde volver.
//
// Pide la configuracion el mismo: quien lo usa no tiene que saber de settings.
// Si no llega, pinta solo el contenido (useSitio).
import CabeceraDelSitio from './CabeceraDelSitio.vue'
import PieDelSitio from './PieDelSitio.vue'
import { useSitio } from '../composables/useSitio'

const sitio = await useSitio()

// La ruta que ve el navegador: el dashboard es del mismo origen y la imagen se
// pide desde el visitante, no desde el SSR.
const base = useRuntimeConfig().public.apiBase as string
const urlDeLogo = (id: string) => `${base}/public/media/${id}`
</script>

<template>
  <CabeceraDelSitio v-if="sitio" :sitio="sitio" :url-de-logo="urlDeLogo" />
  <slot />
  <PieDelSitio v-if="sitio" :sitio="sitio" />
</template>
