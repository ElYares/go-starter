<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'neutral' | 'accent' | 'ok' | 'danger'
    // El color no es informacion. Una pildora roja no dice "error" a quien no
    // distingue rojo de verde, ni a un lector de pantalla. Cuando el estado solo
    // se comunica por color, se pasa un texto aqui y va oculto a la vista.
    srLabel?: string
  }>(),
  { variant: 'neutral' },
)
</script>

<template>
  <span class="pildora" :class="`v-${variant}`">
    <span v-if="srLabel" class="solo-lector">{{ srLabel }}: </span>
    <slot />
  </span>
</template>

<style scoped>
.pildora {
  display: inline-flex;
  align-items: center;
  padding: var(--space-1) var(--space-3);
  border: 1px solid transparent;
  border-radius: var(--radius-full);
  font-size: var(--text-sm);
  font-weight: 600;
  line-height: 1.4;
}

/* Rellenos suaves con el texto en el color fuerte, y no relleno saturado con
   texto blanco: una pildora de color pleno compite con el boton principal y la
   pantalla deja de tener una sola accion evidente. */
.v-neutral {
  background: var(--color-surface-hover);
  color: var(--color-text-muted);
  border-color: var(--color-border);
}
.v-accent {
  background: var(--color-surface-hover);
  color: var(--color-accent-strong);
  border-color: var(--color-accent-strong);
}
.v-ok {
  background: var(--color-surface-hover);
  color: var(--color-ok);
  border-color: var(--color-ok);
}
.v-danger {
  background: var(--color-surface-hover);
  color: var(--color-danger);
  border-color: var(--color-danger);
}

/* No es display:none: eso lo esconde tambien del lector de pantalla, que es
   justo para quien esta el texto. */
.solo-lector {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
</style>
