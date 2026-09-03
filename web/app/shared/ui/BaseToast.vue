<script setup lang="ts">
import {
  ToastClose,
  ToastDescription,
  ToastProvider,
  ToastRoot,
  ToastTitle,
  ToastViewport,
} from 'reka-ui'

// Un aviso no es un <div> que aparece. Reka aporta lo que cuesta: la region
// aria-live correcta segun urgencia, que el temporizador se pause al pasar el
// raton o al enfocar dentro, y que Escape lo cierre. Sin eso, un aviso que se va
// solo en cinco segundos es inservible para quien lee despacio.
//
// Este primitivo se lleva su propio ToastProvider y su Viewport para que un
// aviso suelto funcione sin ceremonia. El dia que un fork necesite una COLA de
// avisos, el provider sube al layout y este componente se queda con el Root: es
// un cambio de diez lineas, y ninguna vista se entera.
withDefaults(
  defineProps<{
    title: string
    description?: string
    variant?: 'neutral' | 'ok' | 'danger'
    duration?: number
  }>(),
  { variant: 'neutral', duration: 5000 },
)

const abierto = defineModel<boolean>('open', { default: false })
</script>

<template>
  <!-- Las etiquetas por omision de Reka estan en ingles ("Notification",
       "Notifications ({hotkey})") y son justo las que se leen en voz alta. En
       una aplicacion en espanol, quien usa lector de pantalla oiria el aviso
       anunciado en otro idioma. No se ve en pantalla, asi que no lo caza nadie
       mirando. -->
  <ToastProvider label="Aviso">
    <ToastRoot
      v-model:open="abierto"
      :duration="duration"
      class="aviso"
      :class="`v-${variant}`"
    >
      <ToastTitle class="titulo">{{ title }}</ToastTitle>
      <ToastDescription v-if="description" class="descripcion">
        {{ description }}
      </ToastDescription>
      <ToastClose class="cerrar" aria-label="Cerrar aviso">×</ToastClose>
    </ToastRoot>
    <ToastViewport class="region" label="Avisos ({hotkey})" />
  </ToastProvider>
</template>

<style scoped>
.region {
  position: fixed;
  right: var(--space-4);
  bottom: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  width: min(24rem, calc(100vw - var(--space-8)));
  margin: 0;
  padding: 0;
  list-style: none;
}

.aviso {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: var(--space-1) var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-left-width: var(--space-1);
  border-radius: var(--radius-md);
}

.v-ok {
  border-left-color: var(--color-ok);
}
.v-danger {
  border-left-color: var(--color-danger);
}

.titulo {
  margin: 0;
  font-size: var(--text-base);
  font-weight: 600;
}

.descripcion {
  grid-column: 1;
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.cerrar {
  grid-row: 1;
  grid-column: 2;
  align-self: start;
  padding: 0 var(--space-2);
  background: none;
  border: none;
  color: var(--color-text-muted);
  font: inherit;
  font-size: var(--text-lg);
  line-height: 1;
  cursor: pointer;
}

.cerrar:hover {
  color: var(--color-text);
}

.cerrar:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: var(--focus-ring);
}
</style>
