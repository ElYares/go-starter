<script setup lang="ts">
import {
  SelectContent,
  SelectItem,
  SelectItemIndicator,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
} from 'reka-ui'
import { computed } from 'vue'

// Un <select> nativo no se puede estilar por dentro de forma consistente entre
// navegadores, y rehacerlo a mano significa reimplementar teclado (flechas,
// inicio/fin, buscar escribiendo), roles ARIA y colocacion del panel. Reka trae
// todo eso; aqui solo va la piel y la forma de las opciones.
export interface OpcionSelect {
  value: string
  label: string
  disabled?: boolean
}

const props = defineProps<{
  id?: string
  options: OpcionSelect[]
  placeholder?: string
  disabled?: boolean
  invalid?: boolean
  describedBy?: string
}>()

const modelo = defineModel<string>()

// La etiqueta se busca en `options` y no se deja en manos de SelectValue.
//
// SelectValue resuelve el texto a partir de los SelectItem REGISTRADOS, y los
// items viven dentro del Portal: hasta que alguien abre el panel una primera
// vez, no hay ninguno registrado. Un formulario precargado desde el servidor
// mostraria "Elegir…" teniendo valor, y el sintoma solo aparece al editar algo
// que ya existe — nunca al crear, que es como se prueba a mano.
const etiqueta = computed(() => props.options.find((o) => o.value === modelo.value)?.label)

const marcador = computed(() => props.placeholder ?? 'Elegir…')
</script>

<template>
  <SelectRoot v-model="modelo" :disabled="disabled">
    <SelectTrigger
      :id="id"
      class="disparador"
      :class="{ mal: invalid }"
      :aria-invalid="invalid || undefined"
      :aria-describedby="describedBy"
    >
      <SelectValue class="valor" :class="{ 'es-marcador': !etiqueta }">
        {{ etiqueta ?? marcador }}
      </SelectValue>
      <span class="punta" aria-hidden="true">▾</span>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent class="panel" position="popper" :side-offset="4">
        <SelectViewport>
          <SelectItem
            v-for="opcion in options"
            :key="opcion.value"
            :value="opcion.value"
            :disabled="opcion.disabled"
            class="opcion"
          >
            <SelectItemText>{{ opcion.label }}</SelectItemText>
            <SelectItemIndicator class="marca">✓</SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>

<style scoped>
.disparador {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font: inherit;
  font-size: var(--text-base);
  cursor: pointer;
}

.disparador:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: 0;
}

.disparador[data-disabled] {
  background: var(--color-surface-hover);
  color: var(--color-text-muted);
  cursor: not-allowed;
}

.mal {
  border-color: var(--color-danger);
}

.valor {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.es-marcador,
.punta {
  color: var(--color-text-muted);
}

.panel {
  min-width: var(--reka-select-trigger-width);
  padding: var(--space-1);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.opcion {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--color-text);
  cursor: pointer;
}

/* data-highlighted es la opcion bajo el cursor O bajo las flechas del teclado.
   Usar :hover dejaria la navegacion por teclado sin ninguna senal visible. */
.opcion[data-highlighted] {
  background: var(--color-accent-strong);
  color: var(--color-on-accent);
  outline: none;
}

.opcion[data-disabled] {
  color: var(--color-text-muted);
  cursor: not-allowed;
}

.marca {
  font-size: var(--text-sm);
}
</style>
