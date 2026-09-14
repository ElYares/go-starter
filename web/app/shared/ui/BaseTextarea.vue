<script setup lang="ts">
// El gemelo de BaseInput para texto largo. Mismas props y la misma piel, para
// que BaseField le reparta id, descripcion e invalido sin saber cual de los dos
// tiene dentro.
withDefaults(
  defineProps<{
    id?: string
    placeholder?: string
    disabled?: boolean
    invalid?: boolean
    describedBy?: string
    rows?: number
  }>(),
  { rows: 4 },
)

const modelo = defineModel<string>({ default: '' })
</script>

<template>
  <textarea
    :id="id"
    v-model="modelo"
    :rows="rows"
    :placeholder="placeholder"
    :disabled="disabled"
    :aria-invalid="invalid || undefined"
    :aria-describedby="describedBy"
    class="entrada"
    :class="{ mal: invalid }"
  />
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
  resize: vertical;
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
