<script setup lang="ts">
// Asignar una contrasena temporal a una cuenta (HU-019). Lo usan la lista de
// solicitudes y la ficha de una cuenta.
//
// Abre con una ya generada y a la vista: lo normal es aceptarla, copiarla y
// pasarsela a la persona. Despues de asignarla se sigue mostrando, porque es el
// unico momento en que alguien la puede copiar; el api solo guarda su hash.
import { ref, watch } from 'vue'
import { BaseButton, BaseDialog, BaseField } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import CampoDeContrasena from '~/shared/formularios/CampoDeContrasena.vue'
import { generarContrasena } from '~/shared/formularios/contrasena'
import { erroresPorCampo } from '~/shared/formularios/esquema'

const props = defineProps<{
  cuenta: { email: string; displayName: string }
  asignar: (password: string) => Promise<void>
}>()

const abierto = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ asignada: [] }>()

const password = ref('')
const asignando = ref(false)
const asignada = ref(false)
const error = ref<string | undefined>()
const fallo = ref<{ mensaje: string; traceId?: string } | null>(null)

// Cada vez que abre, una nueva: reusar la anterior seria darle a una persona la
// contrasena que se genero para otra.
//
// `immediate` porque la lista de solicitudes monta el dialogo ya abierto: sin
// el, el watch nunca ve el paso de cerrado a abierto y el campo sale vacio.
watch(
  abierto,
  (si) => {
    if (!si) return
    password.value = generarContrasena()
    asignada.value = false
    error.value = undefined
    fallo.value = null
  },
  { immediate: true },
)

async function onAsignar() {
  asignando.value = true
  error.value = undefined
  fallo.value = null
  try {
    await props.asignar(password.value)
    asignada.value = true
    emit('asignada')
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      error.value = erroresPorCampo(causa).password ?? causa.message
    } else if (causa.status === 403 || causa.status === 409) {
      // El 403 de "tiene mas poder que tu" y el 409 de "es tu cuenta" traen un
      // mensaje que dice que hacer.
      fallo.value = { mensaje: causa.message, traceId: causa.traceId }
    } else {
      fallo.value = {
        mensaje: causa.unavailable ? 'El servidor no responde. No se asigno nada.' : 'No se pudo asignar la contrasena.',
        traceId: causa.traceId,
      }
    }
  } finally {
    asignando.value = false
  }
}
</script>

<template>
  <BaseDialog
    v-model:open="abierto"
    :title="asignada ? 'Contrasena asignada' : 'Asignar contrasena temporal'"
    :description="
      asignada
        ? `Copiala y pasasela a ${cuenta.displayName} por un canal seguro. No se vuelve a mostrar.`
        : `${cuenta.displayName} (${cuenta.email}) tendra que cambiarla al entrar. Se cerraran sus sesiones abiertas.`
    "
  >
    <form class="dialogo" @submit.prevent="onAsignar">
      <BaseField label="Contrasena temporal" :error="error" hint="Minimo 12 caracteres." required>
        <template #default="{ id, describedBy, invalid }">
          <CampoDeContrasena
            :id="id"
            v-model="password"
            :described-by="describedBy"
            :invalid="invalid"
            generador
            mostrada
            data-campo="password"
          />
        </template>
      </BaseField>
      <div v-if="fallo" class="fallo" role="alert" data-aviso="error">
        <p>{{ fallo.mensaje }}</p>
        <p v-if="fallo.traceId" class="pista">Referencia: <code>{{ fallo.traceId }}</code></p>
      </div>
    </form>
    <template #acciones>
      <template v-if="asignada">
        <BaseButton @click="abierto = false">Listo</BaseButton>
      </template>
      <template v-else>
        <BaseButton variant="secondary" @click="abierto = false">Cancelar</BaseButton>
        <BaseButton :loading="asignando" @click="onAsignar">Asignar</BaseButton>
      </template>
    </template>
  </BaseDialog>
</template>

<style scoped>
.dialogo {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.fallo p {
  margin: 0;
}
.fallo {
  color: var(--color-danger);
  font-size: var(--text-sm);
}
.pista {
  color: var(--color-text-muted);
}
</style>
