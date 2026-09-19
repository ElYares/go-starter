<script setup lang="ts">
import FichaDeCuenta from '../components/FichaDeCuenta.vue'
import {
  darRol,
  deshabilitarCuenta,
  guardarCuenta,
  habilitarCuenta,
  leerCuenta,
  listarRoles,
  quitarRol,
} from '../api'
import { useSesion } from '~/modules/auth/composables/useSesion'
import { usePedirConSesion } from '~/modules/auth/composables/usePedirConSesion'
import { useApi } from '~/shared/api/useApi'
import type { Schemas } from '~/shared/api/generated'

const props = defineProps<{ id: string }>()

useHead({ title: 'Cuenta · Usuarios · go-starter' })

const pedir = usePedirConSesion()
const { perfil } = useSesion()

// Ocultar los controles es conveniencia: la autorizacion real es el 403 del
// servidor, que la ficha tambien sabe mostrar.
const permisos = computed(() => perfil.value?.permissions ?? [])
const puedeEscribir = computed(() => permisos.value.includes('identity.user.write'))
const puedeAsignar = computed(() => permisos.value.includes('identity.role.assign'))
const puedeVerRoles = computed(() => permisos.value.includes('identity.role.read'))
const esPropia = computed(() => perfil.value?.id === props.id)

const cargar = () => pedir(() => leerCuenta(useApi(), props.id))
const guardar = (version: number, cuerpo: Schemas['CuentaModificacion']) =>
  pedir(() => guardarCuenta(useApi(), props.id, version, cuerpo))
const deshabilitar = () => pedir(() => deshabilitarCuenta(useApi(), props.id))
const habilitar = () => pedir(() => habilitarCuenta(useApi(), props.id))
const dar = (rol: string) => pedir(() => darRol(useApi(), props.id, rol))
const quitar = (rol: string) => pedir(() => quitarRol(useApi(), props.id, rol))
const cargarRoles = () => pedir(() => listarRoles(useApi()))

// Salir con cambios sin guardar pregunta, como en el editor de paginas.
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
    <NuxtLink to="/admin/usuarios" class="volver">← Usuarios</NuxtLink>
    <FichaDeCuenta
      :cargar="cargar"
      :guardar="guardar"
      :deshabilitar="deshabilitar"
      :habilitar="habilitar"
      :dar-rol="dar"
      :quitar-rol="quitar"
      :cargar-roles="puedeVerRoles ? cargarRoles : undefined"
      :puede-escribir="puedeEscribir"
      :puede-asignar="puedeAsignar"
      :es-propia="esPropia"
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
