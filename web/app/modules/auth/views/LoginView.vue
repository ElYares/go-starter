<script setup lang="ts">
import LoginForm from '../components/LoginForm.vue'
import { useSesion } from '../composables/useSesion'
import { destinoSeguro, iniciarSesion, type Credenciales } from '../sesion'
import { RUTA_PEDIR_CONTRASENA } from '../contrasena'
import { useApi } from '~/shared/api/useApi'

useHead({ title: 'Entrar · go-starter' })

const route = useRoute()
const { perfil } = useSesion()

// El perfil se guarda ANTES de navegar: el guard de /admin lo encuentra y deja
// pasar sin pedir `me` otra vez.
async function entrar(credenciales: Credenciales) {
  perfil.value = await iniciarSesion(useApi(), credenciales)
  await navigateTo(destinoSeguro(route.query.next), { replace: true })
}
</script>

<template>
  <main class="pantalla">
    <section class="tarjeta">
      <h1>Entrar</h1>
      <p class="pista">Accede al dashboard para editar la landing.</p>
      <LoginForm :entrar="entrar" />
      <NuxtLink :to="RUTA_PEDIR_CONTRASENA" class="olvido">¿Olvidaste tu contrasena?</NuxtLink>
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
.olvido {
  display: inline-block;
  margin-top: var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.pista {
  margin: var(--space-1) 0 var(--space-6);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
</style>
