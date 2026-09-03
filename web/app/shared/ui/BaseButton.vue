<script setup lang="ts">
// Los imports de 'vue' son explicitos a proposito. Nuxt los auto-importaria,
// pero entonces estos componentes solo compilarian dentro de Nuxt y sus pruebas
// necesitarian levantar el arnes completo. Ver el comentario de vitest.config.ts.
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
    size?: 'sm' | 'md'
    // El atributo type por omision de <button> es 'submit'. Dentro de un
    // formulario, eso convierte cualquier boton auxiliar en un envio accidental.
    // Es un error clasico y silencioso, asi que aqui el defecto es 'button' y
    // quien quiera enviar lo pide.
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    loading?: boolean
  }>(),
  { variant: 'primary', size: 'md', type: 'button', disabled: false, loading: false },
)

// Mientras carga, el boton sigue siendo un boton para el lector de pantalla
// (aria-busy) pero no admite un segundo clic. Ocultarlo o desmontarlo mueve el
// foco al body y quien navega con teclado pierde el sitio.
const bloqueado = computed(() => props.disabled || props.loading)
</script>

<template>
  <button
    :type="type"
    :class="['boton', `v-${variant}`, `t-${size}`]"
    :disabled="bloqueado"
    :aria-busy="loading || undefined"
  >
    <span v-if="loading" class="girando" aria-hidden="true" />
    <slot />
  </button>
</template>

<style scoped>
.boton {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  font: inherit;
  font-weight: 600;
  line-height: 1.2;
  cursor: pointer;
}

.t-sm {
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-sm);
}
.t-md {
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-base);
}

.v-primary {
  background: var(--color-accent);
  color: var(--color-on-accent);
}
.v-primary:hover:not(:disabled) {
  background: var(--color-accent-strong);
}

.v-secondary {
  background: var(--color-surface);
  color: var(--color-text);
  border-color: var(--color-border);
}
.v-secondary:hover:not(:disabled) {
  background: var(--color-surface-hover);
}

.v-ghost {
  background: transparent;
  color: var(--color-accent-strong);
}
.v-ghost:hover:not(:disabled) {
  background: var(--color-surface-hover);
}

.v-danger {
  background: var(--color-danger);
  color: var(--color-on-accent);
}

/* :focus-visible y no :focus: con :focus el anillo aparece tambien al hacer
   clic, y la reaccion de todo el mundo es quitarlo del CSS. Entonces se pierde
   para quien navega con teclado, que es justo quien lo necesita. */
.boton:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: var(--focus-ring);
}

.boton:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.girando {
  width: var(--space-3);
  height: var(--space-3);
  border: var(--focus-ring) solid currentColor;
  border-top-color: transparent;
  border-radius: var(--radius-full);
  animation: girar 0.6s linear infinite;
}

@keyframes girar {
  to {
    transform: rotate(360deg);
  }
}

/* Quien pidio menos movimiento no quiere una ruleta girando. El indicador se
   queda quieto en vez de desaparecer: sigue diciendo "esto esta ocupado". */
@media (prefers-reduced-motion: reduce) {
  .girando {
    animation: none;
  }
}
</style>
