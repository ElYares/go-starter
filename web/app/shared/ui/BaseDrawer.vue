<script setup lang="ts">
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
} from 'reka-ui'
import BaseButton from './BaseButton.vue'

// Un panel que entra por la izquierda sobre un velo: el menu de un dashboard
// en angosto. Por dentro es el mismo dialogo de Reka que BaseDialog, con lo
// mismo gratis (Decision 005): foco atrapado, Escape, el resto inerte y el foco
// de vuelta en el boton que lo abrio al cerrar.
//
// El boton que lo abre va en el slot `trigger`, para que Reka lo conozca y
// sepa a donde devolver el foco. El titulo es obligatorio por la misma razon
// que en BaseDialog: sin el, el lector anuncia "dialogo" y nada mas.
withDefaults(
  defineProps<{
    title: string
    closeLabel?: string
  }>(),
  { closeLabel: 'Cerrar' },
)

const abierto = defineModel<boolean>('open', { default: false })
</script>

<template>
  <DialogRoot v-model:open="abierto">
    <DialogTrigger as-child>
      <slot name="trigger" />
    </DialogTrigger>

    <DialogPortal>
      <DialogOverlay class="velo" />
      <!-- Sin descripcion: se quita el aria-describedby que Reka pone siempre,
           como en BaseDialog, para no apuntar a un id que no existe. -->
      <DialogContent class="cajon" v-bind="{ 'aria-describedby': undefined }">
        <div class="cabeza">
          <DialogTitle class="titulo">{{ title }}</DialogTitle>
          <DialogClose as-child>
            <BaseButton variant="ghost" size="sm" :aria-label="closeLabel">
              <svg class="icono" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M6 6l12 12M18 6 6 18" />
              </svg>
            </BaseButton>
          </DialogClose>
        </div>

        <slot />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.velo {
  position: fixed;
  inset: 0;
  background: var(--color-scrim);
}

.cajon {
  position: fixed;
  inset: 0 auto 0 0;
  width: min(18rem, 85vw);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  background: var(--color-surface);
  color: var(--color-text);
  border-right: 1px solid var(--color-border);
}
.cajon[data-state='open'] {
  animation: entrar 160ms ease-out;
}

.cabeza {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 3.5rem;
  padding: 0 var(--space-2) 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.titulo {
  margin: 0;
  font-size: var(--text-base);
  font-weight: 700;
  white-space: nowrap;
}

.icono {
  width: 1.25rem;
  height: 1.25rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

@keyframes entrar {
  from {
    transform: translateX(-100%);
  }
}
@media (prefers-reduced-motion: reduce) {
  .cajon[data-state='open'] {
    animation: none;
  }
}
</style>
