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

/** Las dos cabeceras con las que el SSR le dice al api a quien atiende (HU-010). */
export const CABECERA_SECRETO_SSR = 'X-SSR-Secret'
export const CABECERA_IP_DEL_SSR = 'X-SSR-Client-IP'

/**
 * La IP del visitante que el SSR esta atendiendo, vista desde Nuxt.
 *
 * Nuxt esta detras del edge igual que el api, asi que vale la misma regla que
 * `httpx.ClientIP`: **la ULTIMA entrada de `X-Forwarded-For`**, la que agrego
 * Caddy. La primera la escribe el cliente, y tomarla le dejaria elegir contra
 * que IP cuentan sus visitas. `getRequestIP(event, { xForwardedFor: true })` de
 * h3 toma la primera, por eso no se usa.
 */
export function ipDelVisitante(forwardedFor: string | undefined, conexion: string | undefined): string | undefined {
  const ultima = forwardedFor?.split(',').at(-1)?.trim()
  return ultima || conexion || undefined
}

/**
 * Las cabeceras que acreditan una peticion del SSR ante el api. Sin secreto o
 * sin IP no se manda nada: el api contaria la peticion contra el contenedor
 * web, que es peor, pero mandar media acreditacion no la arregla.
 */
export function cabecerasDelSSR(secreto: string | undefined, ip: string | undefined): Record<string, string> {
  if (!secreto || !ip) return {}
  return { [CABECERA_SECRETO_SSR]: secreto, [CABECERA_IP_DEL_SSR]: ip }
}
