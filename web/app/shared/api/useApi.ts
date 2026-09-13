import { candadoDelNavegador, crearCliente, type ClienteApi } from './client'

let cliente: ClienteApi | undefined

/**
 * El cliente del navegador, armado una vez.
 *
 * Es solo de cliente a proposito: lee `document.cookie` para el CSRF, y en el
 * servidor de Nuxt no hay navegador ni cookies que leer. Lo que se renderiza en
 * servidor —la landing— sigue usando `useApiFetch`, que no manda mutaciones.
 * Por eso esto lanza en SSR en vez de devolver algo que falle despues con un
 * `document is not defined` lejos de la causa.
 */
export function useApi(): ClienteApi {
  if (import.meta.server) {
    throw new Error('useApi es solo de cliente: en SSR usa useApiFetch')
  }

  cliente ??= crearCliente({
    base: useRuntimeConfig().public.apiBase as string,
    fetch: globalThis.fetch.bind(globalThis),
    cookies: () => document.cookie,
    candado: candadoDelNavegador(),
  })
  return cliente
}
