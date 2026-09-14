<script setup lang="ts">
import EditorDePagina from '../components/EditorDePagina.vue'
import { borrarPagina, guardarPagina, leerPagina, listarVersiones, publicarVersion } from '../api'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

const props = defineProps<{ id: string }>()

useHead({ title: 'Editar pagina · go-starter' })

const pedir = usePedirConSesion()
const { perfil } = useSesion()

// Ocultar los botones es conveniencia (CU-004 A1): la autorizacion real es el
// 403 del servidor, que el editor tambien sabe mostrar.
const permisos = computed(() => perfil.value?.permissions ?? [])
const puedeEscribir = computed(() => permisos.value.includes('content.page.write'))
const puedePublicar = computed(() => permisos.value.includes('content.page.publish'))

const cargar = () => pedir(() => leerPagina(useApi(), props.id))
const guardar = (version: number, cuerpo: Schemas['PaginaModificacion']) =>
  pedir(() => guardarPagina(useApi(), props.id, version, cuerpo))
const publicar = (versionId: string) => pedir(() => publicarVersion(useApi(), props.id, versionId))
const borrar = () => pedir(() => borrarPagina(useApi(), props.id))
const cargarVersiones = () => pedir(() => listarVersiones(useApi(), props.id))

// Salir con cambios sin guardar pregunta antes, dentro del dashboard y al
// cerrar la pestana. Perder media hora de edicion por un clic en el menu es
// justo lo que el editor existe para evitar.
const pendientes = ref(false)
let borrada = false

onBeforeRouteLeave(() => {
  if (pendientes.value && !borrada) {
    return window.confirm('Hay cambios sin guardar. ¿Salir de todos modos?')
  }
})

function alCerrar(e: BeforeUnloadEvent) {
  if (pendientes.value) e.preventDefault()
}
onMounted(() => window.addEventListener('beforeunload', alCerrar))
onBeforeUnmount(() => window.removeEventListener('beforeunload', alCerrar))

async function alBorrar() {
  borrada = true
  await navigateTo('/admin/paginas', { replace: true })
}
</script>

<template>
  <EditorDePagina
    :cargar="cargar"
    :guardar="guardar"
    :publicar="publicar"
    :borrar="borrar"
    :cargar-versiones="cargarVersiones"
    :puede-escribir="puedeEscribir"
    :puede-publicar="puedePublicar"
    @cambios="pendientes = $event"
    @borrada="alBorrar"
  />
</template>
