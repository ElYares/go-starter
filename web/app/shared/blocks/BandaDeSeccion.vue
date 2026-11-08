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
@media (min-width: 48rem) {
  .amplia {
    padding-block: var(--space-24);
  }
}
.tono-suave {
  background: var(--color-surface);
  border-block: 1px solid var(--color-border);
}
.tono-marca {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  color: var(--color-on-brand);
  background:
    radial-gradient(60rem 28rem at 85% -10%, color-mix(in srgb, var(--color-brand-glow) 45%, transparent), transparent 70%),
    radial-gradient(40rem 24rem at 5% 110%, color-mix(in srgb, var(--color-brand-glow) 25%, transparent), transparent 70%),
    linear-gradient(180deg, var(--color-brand), var(--color-brand-deep));
}
/* Un anillo concentrico, como el circulo de la imagen de acceso. Es decoracion
   pura: un pseudo-elemento, asi que nadie lo lee. */
.tono-marca::before {
  content: '';
  position: absolute;
  inset: auto -10% -40% auto;
  z-index: -1;
  width: 36rem;
  aspect-ratio: 1;
  border: 1px solid var(--color-brand-line);
  border-radius: var(--radius-full);
  box-shadow: 0 0 0 4rem var(--color-brand-line);
  pointer-events: none;
}
.contenedor {
  max-width: var(--container-w);
  margin-inline: auto;
}
</style>
