<script setup lang="ts">
// Esta ruta es SPA por routeRules: no se renderiza en servidor.
// El guard de sesion llega en CU-003; hoy no hay nada que proteger.
useHead({ title: 'Dashboard · go-starter' })
</script>

<template>
  <main class="admin">
    <p class="eyebrow">fase 0</p>
    <h1>Dashboard</h1>
    <p>
      Renderizado en el cliente. Aqui viviran el editor de la landing (fase 3)
      y las secciones que agregue cada fork.
    </p>
    <p class="hint">
      Sin SSR a proposito: nadie indexa un panel privado, y renderizarlo en
      servidor obligaria a reenviar cookies de sesion en cada navegacion.
    </p>
    <NuxtLink to="/">Volver a la landing</NuxtLink>
  </main>
</template>

<style scoped>
.admin {
  max-width: 46rem;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
}
.eyebrow {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--color-text-muted);
  margin: 0;
}
.hint {
  color: var(--color-text-muted);
  font-size: 0.9rem;
}
</style>
