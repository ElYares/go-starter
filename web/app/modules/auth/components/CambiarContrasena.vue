<script setup lang="ts">
// Cambiar la propia contrasena (HU-019). Sin Nuxt: recibe el envio como
// funcion.
//
// Con una contrasena temporal es la unica pantalla que sirve: el api no le
// resuelve permisos a la cuenta hasta que elija la suya. Por eso el texto lo
// dice, en vez de parecer una opcion mas del perfil.
import { computed, ref } from 'vue'
import { BaseButton, BaseField } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import CampoDeContrasena from '~/shared/formularios/CampoDeContrasena.vue'
import { erroresPorCampo } from '~/shared/formularios/esquema'

const props = defineProps<{
  cambiar: (cuerpo: Schemas['CambioDeContrasena']) => Promise<void>
  temporal: boolean
}>()

const emit = defineEmits<{ cambiada: [] }>()

const actual = ref('')
const nueva = ref('')
const enviando = ref(false)
const errores = ref<Record<string, string>>({})
const fallo = ref<{ mensaje: string; traceId?: string } | null>(null)

const listo = computed(() => actual.value !== '' && nueva.value !== '')

async function enviar() {
  if (!listo.value || enviando.value) return
  enviando.value = true
  errores.value = {}
  fallo.value = null
  try {
    await props.cambiar({ currentPassword: actual.value, newPassword: nueva.value })
    emit('cambiada')
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      errores.value = erroresPorCampo(causa)
    } else if (causa.status === 429) {
      fallo.value = { mensaje: 'Demasiados intentos seguidos. Espera un minuto y vuelve a probar.' }
    } else {
      fallo.value = {
        mensaje: causa.unavailable ? 'El servidor no responde. Tu contrasena no cambio.' : 'No se pudo cambiar la contrasena.',
        traceId: causa.traceId,
      }
    }
  } finally {
    enviando.value = false
  }
}
</script>

<template>
  <section class="cambiar">
    <h1>{{ temporal ? 'Elige tu contrasena' : 'Cambiar mi contrasena' }}</h1>
    <p v-if="temporal" class="aviso" data-aviso="temporal">
      Entraste con una contrasena temporal que te dio otra persona. Antes de seguir, elige una tuya: hasta entonces no
      puedes usar el dashboard.
    </p>
    <p v-else class="pista">Se cerraran tus sesiones en otros dispositivos. Esta sigue abierta.</p>

    <form class="formulario" novalidate @submit.prevent="enviar">
      <BaseField
        :label="temporal ? 'Contrasena temporal' : 'Contrasena actual'"
        required
        :error="errores.currentPassword"
        v-slot="campo"
      >
        <CampoDeContrasena
          v-model="actual"
          :id="campo.id"
          :described-by="campo.describedBy"
          :invalid="campo.invalid"
          autocomplete="current-password"
          data-campo="currentPassword"
        />
      </BaseField>
      <BaseField label="Contrasena nueva" required hint="Minimo 12 caracteres." :error="errores.newPassword" v-slot="campo">
        <CampoDeContrasena
          v-model="nueva"
          :id="campo.id"
          :described-by="campo.describedBy"
          :invalid="campo.invalid"
          generador
          data-campo="newPassword"
        />
      </BaseField>
      <div v-if="fallo" class="fallo" role="alert">
        <p>{{ fallo.mensaje }}</p>
        <p v-if="fallo.traceId" class="pista">Referencia: <code>{{ fallo.traceId }}</code></p>
      </div>
      <BaseButton type="submit" :loading="enviando" :disabled="!listo">Guardar contrasena</BaseButton>
    </form>
  </section>
</template>

<style scoped>
.cambiar,
.formulario {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  max-width: 28rem;
}
h1 {
  margin: 0;
  font-size: var(--text-lg);
}
p {
  margin: 0;
}
.aviso {
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-accent);
  border-radius: var(--radius-md);
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
