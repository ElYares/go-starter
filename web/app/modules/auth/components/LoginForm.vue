<script setup lang="ts">
// Sin nada de Nuxt, igual que los Base*: recibe `entrar` como prop y no sabe de
// router, de estado global ni de a donde se va despues. Eso lo pone la vista.
// Asi el formulario se prueba con @vue/test-utils, en milisegundos.
import { computed, ref } from 'vue'
import { BaseButton, BaseField, BaseInput } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import { mensajeDeLogin, type Credenciales } from '../sesion'

const props = defineProps<{
  entrar: (credenciales: Credenciales) => Promise<void>
}>()

const email = ref('')
const password = ref('')
const enviando = ref(false)
// Los errores de campo no aparecen hasta el primer envio: gritarle "obligatorio"
// a un campo que nadie ha tocado todavia es ruido.
const intentado = ref(false)
const fallo = ref<ApiError | null>(null)

const errorEmail = computed(() =>
  intentado.value && email.value.trim() === '' ? 'Escribe tu correo' : undefined,
)
const errorPassword = computed(() =>
  intentado.value && password.value === '' ? 'Escribe tu contrasena' : undefined,
)

const mensaje = computed(() => (fallo.value ? mensajeDeLogin(fallo.value) : undefined))

// La referencia solo se ensena cuando sirve para algo: en un 401 o un 429 la
// persona ya sabe que paso, y un codigo raro al lado de "contrasena incorrecta"
// parece un error del sistema.
const referencia = computed(() => {
  const f = fallo.value
  if (!f?.traceId || [401, 403, 429].includes(f.status)) return undefined
  return f.traceId
})

async function enviar() {
  intentado.value = true
  if (errorEmail.value || errorPassword.value || enviando.value) return

  enviando.value = true
  fallo.value = null
  try {
    // El correo va tal cual, sin minusculas: quien garantiza que Ana@casa.com y
    // ana@casa.com son la misma cuenta es el citext de la columna. Tampoco hace
    // falta un trim(): un input type="email" ya quita los espacios de los bordes
    // por especificacion, antes de que el valor llegue aqui.
    await props.entrar({ email: email.value, password: password.value })
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    // La contrasena se borra y el correo se queda: es lo que se vuelve a
    // escribir en el caso comun, que es haberse equivocado en ella.
    password.value = ''
    intentado.value = false
  } finally {
    enviando.value = false
  }
}
</script>

<template>
  <!-- novalidate: la validacion nativa del navegador pinta sus burbujas en el
       idioma del sistema y no pasa por BaseField, asi que el lector de pantalla
       oiria dos mensajes distintos para el mismo campo. -->
  <form class="login" novalidate @submit.prevent="enviar">
    <BaseField label="Correo" required :error="errorEmail" v-slot="campo">
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

    <BaseField label="Contrasena" required :error="errorPassword" v-slot="campo">
      <BaseInput
        v-model="password"
        :id="campo.id"
        :described-by="campo.describedBy"
        :invalid="campo.invalid"
        type="password"
        autocomplete="current-password"
      />
    </BaseField>

    <!-- role="alert" para que se anuncie al aparecer: tras un envio el foco
         sigue en el boton y nadie va a ir a buscar este parrafo. -->
    <div v-if="mensaje" class="fallo" role="alert">
      <p>{{ mensaje }}</p>
      <p v-if="referencia" class="referencia">
        Referencia: <code>{{ referencia }}</code>
      </p>
    </div>

    <BaseButton type="submit" :loading="enviando">Entrar</BaseButton>
  </form>
</template>

<style scoped>
.login {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.fallo {
  padding: var(--space-3);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  color: var(--color-text);
}
.fallo p {
  margin: 0;
}
/* Con `.fallo` delante: `.fallo p` pesa mas que `.referencia` sola y le
   borraria el margen. */
.fallo .referencia {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
code {
  font-family: var(--font-mono);
}
</style>
