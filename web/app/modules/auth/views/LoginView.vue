<script setup lang="ts">
import LoginForm from '../components/LoginForm.vue'
import { useSesion } from '../composables/useSesion'
import { destinoSeguro, iniciarSesion, type Credenciales } from '../sesion'
import { RUTA_PEDIR_CONTRASENA } from '../contrasena'
import { useApi } from '~/shared/api/useApi'
import personaje from '~/assets/acceso/personaje.webp'

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
    <!-- La imagen es del proyecto, no del contenido: un fork cambia el archivo
         de assets/acceso/ y los textos de aqui. alt vacio porque es
         decoracion; lo que dice el panel ya esta en el lema. -->
    <aside class="panel">
      <p class="marca">
        <img class="marca-icono" :src="personaje" alt="" width="800" height="800" />
        go-starter
      </p>
      <img class="personaje" :src="personaje" alt="" width="800" height="800" />
      <div class="lema">
        <p class="lema-titulo">Tu sitio, en tus manos</p>
        <p class="lema-bajada">Edita la landing, publica versiones y decide quien entra.</p>
      </div>
    </aside>

    <section class="acceso">
      <div class="contenido">
        <h1>Inicia sesion</h1>
        <p class="pista">Entra para editar la landing.</p>
        <LoginForm :entrar="entrar" />
        <NuxtLink :to="RUTA_PEDIR_CONTRASENA" class="olvido">¿Olvidaste tu contrasena?</NuxtLink>
      </div>
    </section>
  </main>
</template>

<style scoped>
/* La ventana partida en dos: la imagen ocupa la mitad izquierda de arriba a
   abajo y el formulario la derecha. */
.pantalla {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1fr 1fr;
}

/* El panel: la marca arriba, el personaje completo al centro y el lema abajo,
   sobre un degradado que baja al tono profundo de la marca. sticky para que la
   imagen no se vaya si el formulario crece mas que la ventana. */
.panel {
  position: sticky;
  top: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
  padding: var(--space-8);
  color: var(--color-on-brand);
  background:
    radial-gradient(circle at 50% 45%, var(--color-accent), transparent 55%),
    linear-gradient(to bottom, var(--color-brand) 55%, var(--color-brand-deep));
  overflow: hidden;
}
.marca {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin: 0;
  font-weight: 700;
}
.marca-icono {
  width: 2rem;
  height: 2rem;
}
/* Completo: nunca se recorta, se achica para caber en lo que deja el panel. */
.personaje {
  display: block;
  width: min(70%, 28rem);
  max-height: 60vh;
  height: auto;
  object-fit: contain;
  margin: auto;
}
.lema p {
  margin: 0;
}
.lema-titulo {
  font-size: var(--text-display);
  font-weight: 700;
  line-height: 1.1;
}
.lema-bajada {
  margin-top: var(--space-2);
  color: var(--color-on-brand-muted);
}

.acceso {
  display: grid;
  place-items: center;
  padding: var(--space-8) var(--space-4);
}
.contenido {
  width: 100%;
  max-width: 24rem;
  text-align: center;
}
/* El formulario vuelve a alinear a la izquierda: un campo o un mensaje de
   error centrados se leen peor. */
.contenido :deep(form) {
  text-align: start;
}
h1 {
  margin: 0;
  font-size: var(--text-display);
  line-height: 1.1;
}
.pista {
  margin: var(--space-2) 0 var(--space-8);
  color: var(--color-text-muted);
}
.olvido {
  display: inline-block;
  margin-top: var(--space-6);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

/* En angosto no caben dos mitades: el panel pasa arriba como una franja con la
   marca y el personaje chico, y el lema se va para no empujar el formulario. */
@media (max-width: 48rem) {
  .pantalla {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }
  .panel {
    position: static;
    height: auto;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4);
  }
  .personaje {
    width: 4rem;
    margin: 0;
  }
  /* El personaje ya esta a la derecha de la franja: dos veces sobra. */
  .marca-icono {
    display: none;
  }
  .lema {
    display: none;
  }
  .acceso {
    place-items: start center;
    padding: var(--space-8) var(--space-4);
  }
}
</style>
