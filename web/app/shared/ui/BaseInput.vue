<script setup lang="ts">
defineProps<{
  id?: string
  type?: string
  placeholder?: string
  disabled?: boolean
  // No pinta un borde rojo y ya: pone aria-invalid, que es lo que un lector de
  // pantalla anuncia. El color solo no le dice nada a quien no lo ve.
  invalid?: boolean
  describedBy?: string
}>()

const modelo = defineModel<string>({ default: '' })
</script>

<template>
  <input
    :id="id"
    v-model="modelo"
    :type="type ?? 'text'"
    :placeholder="placeholder"
    :disabled="disabled"
    :aria-invalid="invalid || undefined"
    :aria-describedby="describedBy"
    class="entrada"
    :class="{ mal: invalid }"
  >
</template>

<style scoped>
.entrada {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font: inherit;
  font-size: var(--text-base);
}

.entrada::placeholder {
  color: var(--color-text-muted);
}

.entrada:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: 0;
  border-color: var(--color-accent-strong);
}

.mal {
  border-color: var(--color-danger);
}

.entrada:disabled {
  background: var(--color-surface-hover);
  color: var(--color-text-muted);
  cursor: not-allowed;
}
</style>
