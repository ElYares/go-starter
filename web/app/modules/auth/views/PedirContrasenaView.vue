<script setup lang="ts">
import PedirContrasena from '../components/PedirContrasena.vue'
import { pedirContrasena } from '../contrasena'
import { RUTA_LOGIN } from '../sesion'
import { useApi } from '~/shared/api/useApi'

useHead({ title: 'Recuperar el acceso · go-starter' })

const pedir = (email: string) => pedirContrasena(useApi(), email)
</script>

<template>
  <main class="pantalla">
    <section class="tarjeta">
      <h1>Recuperar el acceso</h1>
      <p class="pista">
        Quien administra el sitio vera tu solicitud y te dara una contrasena temporal. No enviamos correos.
      </p>
      <PedirContrasena :pedir="pedir" />
      <NuxtLink :to="RUTA_LOGIN" class="volver">← Volver a entrar</NuxtLink>
    </section>
  </main>
</template>

<style scoped>
.pantalla {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: var(--space-4);
}
.tarjeta {
  width: 100%;
  max-width: 24rem;
  padding: var(--space-6);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}
h1 {
  margin: 0;
  font-size: var(--text-lg);
}
.pista {
  margin: var(--space-1) 0 var(--space-6);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.volver {
  display: inline-block;
  margin-top: var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
</style>
