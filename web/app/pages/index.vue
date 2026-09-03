<script setup lang="ts">
import { BaseBadge, BaseButton, BaseEmptyState } from '~/shared/ui'
import { useApiFetch } from '~/shared/api/useApiFetch'
import type { Schemas } from '~/shared/api/generated'

// Esta peticion sale desde el SERVIDOR de Nuxt hacia la red interna del
// compose. Es la prueba de la fase 0: si el texto de abajo aparece en el HTML
// que devuelve curl, el cableado completo funciona.
const { data, error, status, refresh } = await useApiFetch<Schemas['Readiness']>('/readyz')

useHead({
  title: 'go-starter',
  meta: [
    {
      name: 'description',
      content: 'Starter de Go + Nuxt: landing publica editable desde un dashboard.',
    },
  ],
})
</script>

<template>
  <main class="landing">
    <BaseBadge variant="accent">fase 1</BaseBadge>
    <h1>go-starter</h1>
    <p class="lead">
      Landing publica renderizada en servidor. En la fase 3 su contenido dejara
      de vivir en este archivo y pasara a la base, editable desde el dashboard.
    </p>

    <section class="card">
      <h2>Estado del backend</h2>

      <!-- Los cuatro estados de docs/07-frontend.md, con los primitivos
           haciendo el trabajo. El de error es el que importa: dice que paso,
           trae boton de reintentar, y no expulsa al usuario de la pagina. -->
      <BaseEmptyState
        v-if="error"
        title="La API no responde"
        description="El proceso de Nuxt esta vivo, asi que lo que fallo esta detras. Revisa `devherd logs`."
      >
        <template #accion>
          <BaseButton :loading="status === 'pending'" @click="refresh()">
            Reintentar
          </BaseButton>
        </template>
      </BaseEmptyState>

      <p v-else class="estado">
        <BaseBadge variant="ok" sr-label="Estado de la API">{{ data?.status }}</BaseBadge>
        <BaseBadge :variant="data?.checks?.db === 'up' ? 'ok' : 'danger'" sr-label="Estado de la base">
          base {{ data?.checks?.db }}
        </BaseBadge>
      </p>

      <p class="hint">
        Este texto se renderizo en el servidor. Compruebalo con
        <code>curl -s http://go-starter.localhost/ | grep -i api</code>.
      </p>
    </section>

    <nav class="links">
      <NuxtLink to="/admin">Dashboard</NuxtLink>
      <a href="/api/v1/docs">API</a>
      <a href="/api/v1/readyz">/readyz</a>
    </nav>
  </main>
</template>

<style scoped>
.landing {
  max-width: 46rem;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
}
h1 {
  margin: var(--space-2) 0;
  font-size: var(--text-display);
}
.lead {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
}
.card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-6);
}
.card h2 {
  margin-top: 0;
  font-size: var(--text-lg);
}
.estado {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.hint {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  margin-bottom: 0;
}
code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}
.links {
  display: flex;
  gap: var(--space-4);
  margin-top: var(--space-6);
}
</style>
