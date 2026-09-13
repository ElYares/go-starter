import { ApiError, errorDesdeRespuesta, errorSinRespuesta } from './errors'

/** El nombre de la cookie y de la cabecera los fija la Decision 007. */
export const COOKIE_CSRF = 'XSRF-TOKEN'
export const CABECERA_CSRF = 'X-XSRF-TOKEN'

// La ruta con la que se siembra la cookie CSRF cuando falta. Cualquier GET que
// atraviese la cadena global la emite; healthz es el mas barato y no depende de
// la base ni de la sesion.
const RUTA_SEMILLA = '/healthz'

export interface OpcionesCliente {
  /** `/api/v1` en el navegador. Ver `base.ts`. */
  base: string
  /** Inyectado para probar sin red. En la app es el `fetch` del navegador. */
  fetch: typeof fetch
  /** Inyectado por la misma razon. En la app es `() => document.cookie`. */
  cookies: () => string
}

export interface ClienteApi {
  get<T>(ruta: string, init?: { signal?: AbortSignal }): Promise<T>
  post<T>(ruta: string, cuerpo?: unknown, init?: { signal?: AbortSignal }): Promise<T>
}

/**
 * Lee una cookie de `document.cookie`.
 *
 * Solo sirve para las dos que son legibles a proposito —`XSRF-TOKEN` y
 * `has_session`—; `at` y `rt` son HttpOnly y el navegador no las muestra.
 */
export function leerCookie(nombre: string, cookies: string): string | undefined {
  for (const par of cookies.split(';')) {
    const i = par.indexOf('=')
    if (i < 0) continue
    if (par.slice(0, i).trim() === nombre) {
      return decodeURIComponent(par.slice(i + 1).trim())
    }
  }
  return undefined
}

/**
 * El cliente del navegador.
 *
 * Hoy tiene UNA etapa de las dos que fija docs/07-frontend.md: la
 * normalizacion a ApiError. **El reintento con refresh no esta, y no es un
 * olvido:** `POST /auth/refresh` no existe todavia —es CU-002—, y un
 * interceptor contra un endpoint que responde 404 convertiria cada 401 en dos
 * peticiones sin ganar nada. Cuando llegue, va ANTES de la normalizacion, con la
 * exclusion de la propia ruta de refresh: sin ella, el 401 del refresh espera su
 * propia promesa y la peticion no vuelve nunca.
 *
 * El token CSRF se lee de la cookie EN CADA PETICION y no se guarda. El login
 * lo rota, y el refresh lo rotara: una copia en memoria manda el viejo y
 * responde 403 justo despues de entrar.
 */
export function crearCliente({ base, fetch: pedir, cookies }: OpcionesCliente): ClienteApi {
  async function tokenCSRF(signal?: AbortSignal): Promise<string | undefined> {
    const actual = leerCookie(COOKIE_CSRF, cookies())
    if (actual) return actual

    // Sin cookie, el primer POST de un visitante nuevo —el login— no tendria
    // nada que copiar y el servidor responderia 403 sin que la persona pueda
    // hacer nada. Pasa siempre en `/login`: la pagina la sirve Nuxt, no Go, asi
    // que abrirla no siembra la cookie.
    await enviar('GET', RUTA_SEMILLA, undefined, signal)
    return leerCookie(COOKIE_CSRF, cookies())
  }

  async function enviar<T>(metodo: string, ruta: string, cuerpo: unknown, signal?: AbortSignal): Promise<T> {
    const cabeceras: Record<string, string> = { Accept: 'application/json, application/problem+json' }

    if (metodo !== 'GET') {
      const token = await tokenCSRF(signal)
      // Sin token no se inventa uno: se manda sin cabecera y el servidor
      // responde 403, que es un error con traceId. Un valor inventado daria el
      // mismo 403 y ademas mentiria en el log.
      if (token) cabeceras[CABECERA_CSRF] = token
    }
    if (cuerpo !== undefined) cabeceras['Content-Type'] = 'application/json'

    let res: Response
    try {
      res = await pedir(base + ruta, {
        method: metodo,
        headers: cabeceras,
        body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
        // Mismo origen: las cookies viajan solas. Nunca 'include', que las
        // mandaria tambien a otro origen si alguien cambia la base.
        credentials: 'same-origin',
        signal,
      })
    } catch (causa) {
      throw errorSinRespuesta(causa)
    }

    if (!res.ok) throw await errorDesdeRespuesta(res)
    if (res.status === 204) return undefined as T

    try {
      return (await res.json()) as T
    } catch {
      // Un 200 que no es JSON no lo manda Go: es un proxy que respondio en su
      // nombre. Para quien llama, eso es que el servidor no esta.
      throw new ApiError({
        status: res.status,
        code: 'UNAVAILABLE',
        message: 'La respuesta no es JSON',
        answered: true,
        unavailable: true,
      })
    }
  }

  return {
    get: (ruta, init) => enviar('GET', ruta, undefined, init?.signal),
    post: (ruta, cuerpo, init) => enviar('POST', ruta, cuerpo, init?.signal),
  }
}
