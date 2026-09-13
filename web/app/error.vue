<script setup lang="ts">
import { computed } from 'vue'
import type { NuxtError } from '#app'
import { BaseButton, BaseEmptyState } from '~/shared/ui'

// La pantalla de error de toda la app. Cumple el estado "error" de
// docs/07-frontend.md: dice que paso, deja reintentar y ensena el traceId, que
// es lo que se busca en el log del servidor.
const props = defineProps<{ error: NuxtError<{ traceId?: string; destino?: string }> }>()

useHead({ title: 'Algo fallo · go-starter' })

const noExiste = computed(() => props.error.statusCode === 404)
const traceId = computed(() => props.error.data?.traceId)

// "Tu sesion no se ha cerrado" solo cuando es cierto que el problema es la
// caida: es lo que evita que alguien crea que perdio su trabajo y se vaya.
const descripcion = computed(() =>
  props.error.statusCode === 503
    ? 'Tu sesion no se ha cerrado. Cuando el servidor vuelva, reintenta.'
    : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.',
)

// Reintentar RECARGA el destino, y no es un capricho: `clearError({ redirect })`
// hacia la ruta en la que ya esta la URL es una navegacion duplicada, Vue
// Router la descarta sin correr ningun middleware, y el error se limpia igual.
// El resultado era el dashboard pintado SIN pasar por el guard y sin perfil.
// Una recarga arranca de cero: si el servidor ya responde se entra, y si no se
// vuelve aqui. `force` porque reloadNuxtApp ignora por omision una segunda
// recarga en diez segundos, y un segundo clic en "Reintentar" no haria nada.
function reintentar() {
  reloadNuxtApp({ path: props.error.data?.destino ?? '/', persistState: false, force: true })
}
</script>

<template>
  <main class="pantalla">
    <BaseEmptyState
      v-if="noExiste"
      title="Esta pagina no existe"
      description="Puede que el enlace este mal escrito o que la pagina se haya quitado."
    >
      <template #accion>
        <BaseButton @click="clearError({ redirect: '/' })">Ir al inicio</BaseButton>
      </template>
    </BaseEmptyState>

    <BaseEmptyState
      v-else
      :title="error.statusMessage || 'Algo fallo'"
      :description="descripcion"
    >
      <template #accion>
        <BaseButton @click="reintentar">Reintentar</BaseButton>
      </template>
    </BaseEmptyState>

    <p v-if="traceId" class="referencia">
      Referencia: <code>{{ traceId }}</code>
    </p>
  </main>
</template>

<style scoped>
.pantalla {
  max-width: 32rem;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
}
.referencia {
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
code {
  font-family: var(--font-mono);
}
</style>
