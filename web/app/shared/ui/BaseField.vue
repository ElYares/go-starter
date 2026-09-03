<script setup lang="ts">
import { computed, useId } from 'vue'
import { Label } from 'reka-ui'

// El pegamento de accesibilidad de todo formulario, en un solo sitio.
//
// Cablear a mano `for`/`id` y `aria-describedby` en cada campo funciona hasta
// que alguien copia un bloque y se lleva el id del vecino: dos etiquetas
// apuntando al mismo input, y ninguna prueba se entera. Aqui el id se genera y
// se reparte por slot, asi que no hay nada que copiar mal.
//
// useId() de Vue 3.5 da un id estable entre servidor y cliente. Un Math.random()
// aqui produciria un desajuste de hidratacion en la landing, que es SSR.
const props = defineProps<{
  label: string
  hint?: string
  error?: string
  required?: boolean
}>()

const id = useId()
const idHint = `${id}-hint`
const idError = `${id}-error`

// El orden importa: el lector lee el error primero. Y si no hay error, la
// referencia no puede quedar apuntando a un elemento que no existe.
const describedBy = computed(
  () => [props.error ? idError : null, props.hint ? idHint : null].filter(Boolean).join(' ') || undefined,
)
</script>

<template>
  <div class="campo">
    <Label :for="id" class="etiqueta">
      {{ label }}
      <span v-if="required" class="obligatorio" aria-hidden="true">*</span>
    </Label>

    <slot :id="id" :described-by="describedBy" :invalid="Boolean(error)" />

    <!-- role="alert" para que el error se anuncie cuando aparece, y no solo
         cuando alguien vuelve a enfocar el campo. -->
    <p v-if="error" :id="idError" class="error" role="alert">{{ error }}</p>
    <p v-else-if="hint" :id="idHint" class="pista">{{ hint }}</p>
  </div>
</template>

<style scoped>
.campo {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}
.etiqueta {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--color-text);
}
.obligatorio {
  color: var(--color-danger);
}
.error {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-danger);
}
.pista {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
</style>
