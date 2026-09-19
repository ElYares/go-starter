<script setup lang="ts">
// "¿Olvidaste tu contrasena?" (HU-019). Sin Nuxt, como LoginForm: recibe el
// envio como funcion.
//
// El mensaje de exito es el que manda el api, y es el mismo exista o no la
// cuenta: aqui no se escribe otro, porque uno distinto segun el caso delataria
// lo que el api se cuida de no decir.
import { ref } from 'vue'
import { BaseButton, BaseField, BaseInput } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { erroresPorCampo } from '~/shared/formularios/esquema'

const props = defineProps<{ pedir: (email: string) => Promise<Schemas['PedidoRecibido']> }>()

const email = ref('')
const enviando = ref(false)
const recibido = ref<string | null>(null)
const errorEmail = ref<string | undefined>()
const fallo = ref<{ mensaje: string; traceId?: string } | null>(null)

async function enviar() {
  errorEmail.value = undefined
  fallo.value = null
  if (!email.value.trim()) {
    errorEmail.value = 'Escribe el correo de tu cuenta'
    return
  }
  enviando.value = true
  try {
    recibido.value = (await props.pedir(email.value.trim())).message
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      errorEmail.value = erroresPorCampo(causa).email ?? 'Revisa el correo'
    } else if (causa.status === 429) {
      fallo.value = { mensaje: 'Demasiados intentos seguidos. Espera un minuto y vuelve a probar.' }
    } else {
      fallo.value = {
        mensaje: causa.unavailable ? 'El servidor no responde. Vuelve a intentarlo.' : 'No se pudo enviar la solicitud.',
        traceId: causa.traceId,
      }
    }
  } finally {
    enviando.value = false
  }
}
</script>

<template>
  <div v-if="recibido" class="recibido" role="status" data-estado="recibido">
    <p>{{ recibido }}</p>
    <p class="pista">Cuando te la den, entra con ella: te pediremos elegir una tuya.</p>
  </div>

  <form v-else class="pedir" novalidate @submit.prevent="enviar">
    <BaseField label="Correo de tu cuenta" required :error="errorEmail" v-slot="campo">
      <BaseInput
        v-model="email"
        :id="campo.id"
        :described-by="campo.describedBy"
        :invalid="campo.invalid"
        type="email"
        autocomplete="username"
        inputmode="email"
      />
    </BaseField>
    <div v-if="fallo" class="fallo" role="alert">
      <p>{{ fallo.mensaje }}</p>
      <p v-if="fallo.traceId" class="pista">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>
    <BaseButton type="submit" :loading="enviando">Pedir una contrasena temporal</BaseButton>
  </form>
</template>

<style scoped>
.pedir,
.recibido {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.recibido p,
.fallo p {
  margin: 0;
}
.fallo {
  color: var(--color-danger);
  font-size: var(--text-sm);
}
.pista {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
</style>
