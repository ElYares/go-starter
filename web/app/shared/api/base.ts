/**
 * La misma peticion sale hacia dos URLs distintas segun donde corre el codigo.
 * Es la clase de bug que una SPA no tiene y el SSR estrena:
 *
 * - en el navegador, mismo origen (`/api/v1`) y el edge enruta
 * - en el servidor de Nuxt, la red interna del compose (`http://api:8080/api/v1`),
 *   porque desde ese contenedor `go-starter.localhost` no existe
 *
 * Equivocarse da un ECONNREFUSED que solo aparece en SSR: la pagina se ve bien
 * al navegar y en blanco al recargar.
 *
 * Esta funcion es pura a proposito, para poder probarla sin levantar Nuxt.
 */
export interface ApiBaseInput {
  isServer: boolean
  internal: string
  publicBase: string
}

export function resolveApiBase({ isServer, internal, publicBase }: ApiBaseInput): string {
  return isServer ? internal : publicBase
}
