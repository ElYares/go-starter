<script setup lang="ts">
import EditorDeSetting from '../components/EditorDeSetting.vue'
import { esquemas } from '../esquemas'
import { guardarSetting, leerSetting, subirMedio } from '../api'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'

const props = defineProps<{ clave: string }>()

useHead({ title: () => `${esquemas[props.clave]?.title ?? props.clave} · Configuracion · go-starter` })

const pedir = usePedirConSesion()
const { perfil } = useSesion()

// Ocultar los botones es conveniencia: la autorizacion real es el 403 del
// servidor, que el editor tambien sabe mostrar.
const permisos = computed(() => perfil.value?.permissions ?? [])
const puedeEscribir = computed(() => permisos.value.includes('settings.write'))
const puedeSubir = computed(() => permisos.value.includes('media.upload'))

const cargar = () => pedir(() => leerSetting(useApi(), props.clave))
const guardar = (version: number, valor: unknown) => pedir(() => guardarSetting(useApi(), props.clave, version, valor))
const subir = (archivo: File) => pedir(() => subirMedio(useApi(), archivo))

// Salir con cambios sin guardar pregunta, dentro del dashboard y al cerrar la
// pestana, como el editor de paginas.
const pendientes = ref(false)

onBeforeRouteLeave(() => {
  if (pendientes.value) return window.confirm('Hay cambios sin guardar. ¿Salir de todos modos?')
})

function alCerrar(e: BeforeUnloadEvent) {
  if (pendientes.value) e.preventDefault()
}
onMounted(() => window.addEventListener('beforeunload', alCerrar))
onBeforeUnmount(() => window.removeEventListener('beforeunload', alCerrar))
</script>

<template>
  <div class="vista">
    <NuxtLink to="/admin/configuracion" class="volver">← Configuracion</NuxtLink>
    <EditorDeSetting
      :clave="clave"
      :cargar="cargar"
      :guardar="guardar"
      :subir-medio="puedeSubir ? subir : undefined"
      :puede-escribir="puedeEscribir"
      @cambios="pendientes = $event"
    />
  </div>
</template>

<style scoped>
.vista {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.volver {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
</style>
