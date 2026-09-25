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

  // El key va explicito porque el de Nuxt incluye el baseURL, y aqui el baseURL
  // cambia entre servidor y navegador: el cliente no encontraba en el payload lo
  // que dejo el SSR, al hidratar no pedia nada (Nuxt lo deja para onBeforeMount)
  // y la landing se cambiaba por "El sitio no esta disponible" con JavaScript.
  return useFetch<T>(path, { baseURL, headers, key: `api:${path}`, ...options })
}
