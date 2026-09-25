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
// Ver lo que se escribio antes de enviar: en un telefono, equivocarse en una
// letra de la contrasena y no poder verla es la mitad de los 401.
const verPassword = ref(false)
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
    <!-- Las etiquetas se ocultan a la vista, no al lector: el icono y el
         placeholder dicen que va en cada campo a quien ve la pantalla. -->
    <BaseField label="Correo" label-oculta required :error="errorEmail" v-slot="campo">
      <div class="con-icono">
        <svg class="icono" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3" y="5" width="18" height="14" rx="2" />
          <path d="m3 7 9 6 9-6" />
        </svg>
        <BaseInput
          v-model="email"
          :id="campo.id"
          :described-by="campo.describedBy"
          :invalid="campo.invalid"
          type="email"
          autocomplete="username"
          inputmode="email"
          placeholder="Tu correo"
        />
      </div>
    </BaseField>

    <BaseField label="Contrasena" label-oculta required :error="errorPassword" v-slot="campo">
      <div class="con-icono con-accion">
        <svg class="icono" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4" y="11" width="16" height="10" rx="2" />
          <path d="M8 11V7a4 4 0 0 1 8 0v4" />
        </svg>
        <BaseInput
          v-model="password"
          :id="campo.id"
          :described-by="campo.describedBy"
          :invalid="campo.invalid"
          :type="verPassword ? 'text' : 'password'"
          autocomplete="current-password"
          placeholder="Contrasena"
        />
        <!-- type="button": dentro de un form, un boton sin tipo envia. -->
        <button
          type="button"
          class="accion"
          :aria-label="verPassword ? 'Ocultar contrasena' : 'Mostrar contrasena'"
          :aria-pressed="verPassword"
          @click="verPassword = !verPassword"
        >
          <svg class="icono-accion" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
            <circle cx="12" cy="12" r="3" />
            <path v-if="verPassword" d="m4 4 16 16" />
          </svg>
        </button>
      </div>
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

/* Los campos del login van rellenos y sin borde, con el icono dentro. El
   input es la raiz de BaseInput, asi que recibe el atributo scoped de aqui y
   estas reglas le llegan sin :deep(). El borde se queda transparente y no se
   quita: es el que se pinta en rojo cuando el campo es invalido. */
.con-icono {
  position: relative;
}
.con-icono input {
  padding: var(--space-3) var(--space-3) var(--space-3) calc(var(--space-8) + var(--space-3));
  background: var(--color-surface-hover);
  border-color: transparent;
  border-radius: var(--radius-md);
}
/* Esta regla pesa mas que la de BaseInput, asi que el rojo del invalido se
   repite aqui o el transparente de arriba lo borra. */
.con-icono input[aria-invalid="true"] {
  border-color: var(--color-danger);
}
.con-accion input {
  padding-right: calc(var(--space-8) + var(--space-3));
}
.icono,
.icono-accion {
  width: 1.25rem;
  height: 1.25rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.icono {
  position: absolute;
  left: var(--space-3);
  top: 50%;
  transform: translateY(-50%);
  color: var(--color-text-muted);
  pointer-events: none;
}
.accion {
  position: absolute;
  right: var(--space-2);
  top: 50%;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  padding: var(--space-1);
  background: transparent;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  cursor: pointer;
}
.accion:hover {
  color: var(--color-text);
}
.accion:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
}

/* El boton principal, mas alto y con un halo de su color. */
.login .boton {
  margin-top: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-md);
  box-shadow: 0 var(--space-2) var(--space-6) calc(var(--space-2) * -1) var(--color-accent);
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
