import { cabecerasDelSSR, ipDelVisitante, resolveApiBase } from './base'

/**
 * Envoltorio minimo de `useFetch` que resuelve la URL base segun donde corre.
 *
 * En SSR ademas acredita la peticion ante el api con la IP del visitante
 * (HU-010). Ver `cabecerasDelSSR`.
 */
export function useApiFetch<T>(path: string, options: Record<string, unknown> = {}) {
  const config = useRuntimeConfig()

  const baseURL = resolveApiBase({
    isServer: import.meta.server,
    internal: config.apiInternal as string,
    publicBase: config.public.apiBase as string,
  })

  // En SSR la peticion sale del contenedor web, no del visitante. Sin estas
  // cabeceras, el limite por IP del api contaria todas las visitas de la
  // landing contra el contenedor, y el primer pico la tumbaria con 429.
  const headers = import.meta.server
    ? cabecerasDelSSR(
        config.apiSsrSecret as string,
        ipDelVisitante(useRequestHeaders(['x-forwarded-for'])['x-forwarded-for'], useRequestEvent()?.node.req.socket.remoteAddress),
      )
    : {}

  return useFetch<T>(path, { baseURL, headers, ...options })
}
