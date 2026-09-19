<script setup lang="ts">
// Un campo de contrasena con mostrar, generar y copiar (HU-019). Lo usan el
// alta de una cuenta, asignar una temporal y cambiar la propia.
//
// Generar la deja a la vista: una contrasena que no se ve no se puede
// comprobar antes de pasarsela a nadie. Copiar existe por lo mismo, para que no
// haya que transcribirla.
import { ref, watch } from 'vue'
import { BaseButton, BaseInput } from '~/shared/ui'
import { generarContrasena } from './contrasena'

const props = withDefaults(
  defineProps<{
    id?: string
    describedBy?: string
    invalid?: boolean
    autocomplete?: string
    /** Si ofrece "Generar". Para la contrasena actual no tiene sentido. */
    generador?: boolean
    /** Empieza a la vista: una temporal ya generada que hay que poder leer. */
    mostrada?: boolean
    /** Solo para probar sin portapapeles del navegador. */
    copiar?: (texto: string) => Promise<void>
  }>(),
  { autocomplete: 'new-password', generador: false, mostrada: false },
)

const valor = defineModel<string>({ default: '' })
const visible = ref(props.mostrada)
const copiada = ref(false)
// "Copiada" habla de lo que hay ahora en el campo: si cambia, ya no.
watch(valor, () => (copiada.value = false))

function generar() {
  valor.value = generarContrasena()
  visible.value = true
}

async function onCopiar() {
  const escribir = props.copiar ?? ((t: string) => navigator.clipboard.writeText(t))
  try {
    await escribir(valor.value)
    copiada.value = true
  } catch {
    // Sin permiso de portapapeles —un contexto sin HTTPS, por ejemplo— queda
    // a la vista para copiarla a mano.
    visible.value = true
  }
}
</script>

<template>
  <div class="contrasena">
    <BaseInput
      :id="id"
      v-model="valor"
      :type="visible ? 'text' : 'password'"
      :described-by="describedBy"
      :invalid="invalid"
      :autocomplete="autocomplete"
      data-campo-contrasena
    />
    <div class="acciones">
      <BaseButton variant="ghost" size="sm" data-accion="mostrar" @click="visible = !visible">
        {{ visible ? 'Ocultar' : 'Mostrar' }}
      </BaseButton>
      <BaseButton v-if="generador" variant="secondary" size="sm" data-accion="generar" @click="generar">Generar</BaseButton>
      <BaseButton v-if="valor" variant="ghost" size="sm" data-accion="copiar" @click="onCopiar">
        {{ copiada ? 'Copiada' : 'Copiar' }}
      </BaseButton>
    </div>
  </div>
</template>

<style scoped>
.contrasena {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.acciones {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
