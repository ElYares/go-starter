<script setup lang="ts">
// Una banda de ancho completo con su contenido centrado. Los bloques de la
// landing la comparten para que el fondo llegue al borde y el texto no. No
// importa nada de Nuxt: un bloque se prueba sin su arnes.
defineProps<{
  /** `marca` es el relleno de la portada; `suave` alterna el ritmo entre bandas. */
  tono?: 'normal' | 'suave' | 'marca'
  /** Una banda mas alta, para la portada. */
  amplia?: boolean
}>()
</script>

<template>
  <section class="banda" :class="[`tono-${tono ?? 'normal'}`, { amplia }]">
    <div class="contenedor"><slot /></div>
  </section>
</template>

<style scoped>
.banda {
  padding-block: var(--space-12);
  padding-inline: var(--space-4);
}
.amplia {
  padding-block: var(--space-16);
}
.tono-suave {
  background: var(--color-surface);
}
.tono-marca {
  background: var(--color-brand);
  color: var(--color-on-brand);
}
.contenedor {
  max-width: var(--container-w);
  margin-inline: auto;
}
</style>
