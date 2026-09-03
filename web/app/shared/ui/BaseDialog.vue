<script setup lang="ts">
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import BaseButton from './BaseButton.vue'

// Este es el componente que justifica la Decision 005 entera.
//
// Un dialogo correcto atrapa el foco dentro mientras esta abierto, lo devuelve
// al elemento que lo abrio al cerrar, cierra con Escape, marca el resto de la
// pagina como inerte y anuncia su titulo. Escrito a mano son doscientas lineas
// que casi nadie prueba, y que hay que mantener en cada fork. Reka lo trae.
//
// Lo que ponemos nosotros es la piel y el titulo obligatorio: un dialogo sin
// DialogTitle deja al lector de pantalla anunciando "dialogo" y nada mas.
defineProps<{
  title: string
  description?: string
}>()

const abierto = defineModel<boolean>('open', { default: false })
</script>

<template>
  <DialogRoot v-model:open="abierto">
    <DialogPortal>
      <DialogOverlay class="velo" />
      <!-- Reka cablea aria-describedby al id de su DialogDescription SIEMPRE,
           haya descripcion o no. Sin descripcion eso deja al dialogo apuntando
           a un elemento que no existe, y un lector de pantalla se queda mudo
           donde deberia leer algo. v-bind vacio cuando si hay: asi no pisamos
           el id que Reka ya puso. -->
      <DialogContent
        class="dialogo"
        v-bind="description ? {} : { 'aria-describedby': undefined }"
      >
        <DialogTitle class="titulo">{{ title }}</DialogTitle>
        <DialogDescription v-if="description" class="descripcion">
          {{ description }}
        </DialogDescription>

        <div class="cuerpo">
          <slot />
        </div>

        <div class="pie">
          <slot name="acciones">
            <!-- as-child: DialogClose no pinta su propio boton, le pega el
                 comportamiento al nuestro. Sin esto habria dos estilos de boton
                 en el proyecto, y el del dialogo se quedaria atras. -->
            <DialogClose as-child>
              <BaseButton variant="secondary">Cerrar</BaseButton>
            </DialogClose>
          </slot>
        </div>
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

.dialogo {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: min(32rem, calc(100vw - var(--space-8)));
  padding: var(--space-6);
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

.titulo {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: 600;
}

.descripcion {
  margin: var(--space-2) 0 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.cuerpo {
  margin-top: var(--space-4);
}

.pie {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-6);
}
</style>
