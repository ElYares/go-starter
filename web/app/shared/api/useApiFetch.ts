import { resolveApiBase } from './base'

/**
 * Envoltorio minimo de `useFetch` que resuelve la URL base segun donde corre.
 *
 * En la fase 1 crece: interceptores de refresh y normalizacion a ApiError, en
 * ese orden, que es contrato. Ver docs/07-frontend.md.
 */
export function useApiFetch<T>(path: string, options: Record<string, unknown> = {}) {
  const config = useRuntimeConfig()

  const baseURL = resolveApiBase({
    isServer: import.meta.server,
    internal: config.apiInternal as string,
    publicBase: config.public.apiBase as string,
  })

  return useFetch<T>(path, { baseURL, ...options })
}
