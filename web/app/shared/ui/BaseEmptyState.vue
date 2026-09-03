<script setup lang="ts">
// El estado que siempre se olvida, y la primera pantalla que ve el dueno de un
// fork recien instalado (docs/07-frontend.md, "los cuatro estados").
//
// Por eso la accion no es opcional por diseno: un vacio que solo dice "no hay
// nada" deja al usuario sin saber si esta roto o si le toca hacer algo. Aqui el
// slot de accion existe siempre; si un fork lo deja sin llenar, se nota.
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div class="vacio">
    <p class="titulo">{{ title }}</p>
    <p v-if="description" class="descripcion">{{ description }}</p>
    <div class="accion">
      <slot name="accion" />
    </div>
  </div>
</template>

<style scoped>
.vacio {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-8) var(--space-4);
  background: var(--color-surface);
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-lg);
  text-align: center;
}

.titulo {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--color-text);
}

.descripcion {
  margin: 0;
  max-width: 34rem;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.accion:not(:empty) {
  margin-top: var(--space-4);
}
</style>
