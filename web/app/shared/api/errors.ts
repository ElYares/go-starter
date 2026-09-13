import type { Schemas } from './generated'

/**
 * La unica forma en que un fallo de la API llega a una vista.
 *
 * Distingue dos cosas que se confunden siempre, y colapsarlas rompe la
 * rehidratacion de sesion (CU-003):
 *
 * - `answered` es el hecho crudo: hubo respuesta del servidor. Un `401` la
 *   tiene en `true`
 * - `unavailable` es la decision, definida en positivo: es una caida. Un `502`
 *   esta contestado y ES caida; una peticion cancelada no esta contestada y NO
 *   lo es
 *
 * Al arrancar, un `401` significa "no hay sesion" —ir al login— y una caida
 * significa "no se" —mostrar error y no expulsar a nadie—.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly traceId?: string
  readonly errors: Schemas['Problem']['errors']
  readonly answered: boolean
  readonly unavailable: boolean
  // En segundos, como la cabecera. Solo viene en un 429.
  readonly retryAfter?: number

  constructor(init: {
    status: number
    code: string
    message: string
    traceId?: string
    errors?: Schemas['Problem']['errors']
    answered: boolean
    unavailable: boolean
    retryAfter?: number
  }) {
    super(init.message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.traceId = init.traceId
    this.errors = init.errors
    this.answered = init.answered
    this.unavailable = init.unavailable
    this.retryAfter = init.retryAfter
  }
}

// Lo que responde el edge cuando el proceso de Go no esta: Caddy da 502 si no
// hay a quien pasarle la peticion, y 503/504 salen de cualquier proxy delante.
const CAIDAS = new Set([502, 503, 504])

/**
 * Traduce una respuesta que no fue 2xx.
 *
 * El cuerpo se intenta leer como `problem+json`, pero no se da por hecho: un
 * `502` lo escribe Caddy, no Go, y trae HTML o nada. Esa respuesta tiene que
 * salir igual como ApiError, con `unavailable` en `true`, y no como un
 * `SyntaxError` de JSON que ninguna vista sabe interpretar.
 */
export async function errorDesdeRespuesta(res: Response): Promise<ApiError> {
  let problema: Partial<Schemas['Problem']> = {}
  if (res.headers.get('content-type')?.includes('json')) {
    try {
      problema = await res.json()
    } catch {
      // Cuerpo roto: se queda el estado HTTP, que es lo unico fiable.
    }
  }

  return new ApiError({
    status: res.status,
    code: problema.code ?? (CAIDAS.has(res.status) ? 'UNAVAILABLE' : 'UNKNOWN'),
    message: problema.detail ?? problema.title ?? `HTTP ${res.status}`,
    traceId: problema.traceId,
    errors: problema.errors,
    answered: true,
    unavailable: CAIDAS.has(res.status),
    retryAfter: segundosDeRetryAfter(res.headers.get('retry-after')),
  })
}

/**
 * Traduce un fallo sin respuesta: `fetch` rechazo.
 *
 * Una cancelacion (`AbortError`) la pidio el propio cliente —el usuario salio
 * de la pantalla— y no dice nada del servidor. Todo lo demas es que no hubo
 * con quien hablar: red caida, DNS, conexion rechazada.
 */
export function errorSinRespuesta(causa: unknown): ApiError {
  const cancelada = causa instanceof DOMException && causa.name === 'AbortError'
  return new ApiError({
    status: 0,
    code: cancelada ? 'ABORTED' : 'UNAVAILABLE',
    message: cancelada ? 'Peticion cancelada' : 'No hubo respuesta del servidor',
    answered: false,
    unavailable: !cancelada,
  })
}

/**
 * `Retry-After` admite segundos o una fecha HTTP. Go manda segundos, pero la
 * cabecera la puede reescribir un proxy, y una fecha mal leida como numero da
 * `NaN` minutos en pantalla.
 */
export function segundosDeRetryAfter(valor: string | null, ahora = Date.now()): number | undefined {
  if (!valor) return undefined
  if (/^\d+$/.test(valor.trim())) return Number(valor.trim())

  const fecha = Date.parse(valor)
  if (Number.isNaN(fecha)) return undefined
  return Math.max(0, Math.ceil((fecha - ahora) / 1000))
}
