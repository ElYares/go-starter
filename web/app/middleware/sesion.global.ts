import { leerCookie } from '~/shared/api/client'
import { useApi } from '~/shared/api/useApi'
import { useSesion } from '~/modules/auth/composables/useSesion'
import {
  COOKIE_PISTA,
  RUTA_LOGIN,
  decidirAcceso,
  esRutaProtegida,
  pedirPerfil,
} from '~/modules/auth/sesion'

/**
 * El guard del dashboard. Global a proposito: un middleware con nombre hay que
 * acordarse de ponerlo en cada pagina, y la pagina que se olvide queda abierta.
 * Asi, una ruta nueva bajo /admin nace protegida.
 *
 * Proteger es conveniencia de interfaz: lo que de verdad niega el acceso es el
 * guard de cada ruta del servidor.
 */
export default defineNuxtRouteMiddleware(async (to) => {
  if (!esRutaProtegida(to.path)) return

  // Una funcion y no el resultado: `navigateTo` fuera de un `return` puede
  // disparar la navegacion en el acto, aunque despues se decida dejar pasar.
  const login = () => navigateTo({ path: RUTA_LOGIN, query: { next: to.fullPath } })

  // /admin no se renderiza en servidor (routeRules), asi que aqui no deberia
  // llegar nunca. Si alguien le quita el `ssr: false`, cierra en vez de abrir:
  // el servidor de Nuxt no tiene las cookies del navegador para preguntar a
  // `me`, y dejar pasar seria renderizar el panel a quien no tiene sesion.
  if (import.meta.server) return login()

  const { perfil } = useSesion()
  const acceso = await decidirAcceso({
    perfil: perfil.value,
    pista: leerCookie(COOKIE_PISTA, document.cookie) !== undefined,
    pedirPerfil: () => pedirPerfil(useApi()),
  })

  switch (acceso.tipo) {
    case 'pasar':
      perfil.value = acceso.perfil
      return
    case 'login':
      return login()
    case 'error':
      // No al login: una caida no es falta de sesion. Va a error.vue, que dice
      // que paso y deja reintentar.
      return abortNavigation(
        createError({
          statusCode: acceso.error.unavailable ? 503 : acceso.error.status || 500,
          statusMessage: acceso.error.unavailable ? 'El servidor no responde' : 'No se pudo cargar la sesion',
          // `destino` es a donde vuelve el boton de reintentar de error.vue.
          data: { traceId: acceso.error.traceId, destino: to.fullPath },
          fatal: true,
        }),
      )
  }
})
