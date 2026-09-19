import { ApiError, errorDesdeRespuesta, errorSinRespuesta } from './errors'

/** El nombre de la cookie y de la cabecera los fija la Decision 007. */
export const COOKIE_CSRF = 'XSRF-TOKEN'
export const CABECERA_CSRF = 'X-XSRF-TOKEN'

/**
 * La pista de sesion (Decision 007). No es una credencial: dice "hubo sesion"
 * para que una carga sin ella no pida nada que ya se sabe que responde 401.
 */
export const COOKIE_PISTA = 'has_session'

export const RUTA_REFRESH = '/auth/refresh'

// Las rutas cuyo 401 NO dispara un refresh.
//
// - el refresh, porque su 401 volveria a entrar aqui, encontraria su propia
//   promesa en vuelo y se pondria a esperarla: una peticion que no vuelve nunca,
//   que en pruebas se ve como timeout y no como fallo
// - el login, porque su 401 son credenciales malas, no una sesion caducada
// - el logout, porque renovar para cerrar no tiene sentido
const SIN_RENOVACION = new Set([RUTA_REFRESH, '/auth/login', '/auth/logout'])

/** El nombre del candado entre pestanas. Uno por origen es suficiente. */
export const CANDADO_REFRESH = 'go-starter:refresh'

export type Candado = (nombre: string, trabajo: () => Promise<void>) => Promise<void>

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
  /**
   * Serializa el refresh entre pestanas. En la app es `navigator.locks`; sin
   * soporte, corre el trabajo directo y queda solo la promesa compartida de la
   * pestana.
   */
  candado?: Candado
}

export interface OpcionesPeticion {
  signal?: AbortSignal
}

export interface OpcionesEscritura extends OpcionesPeticion {
  /**
   * El ETag que devolvio la lectura, tal cual (`"7"`). Es obligatorio en todo
   * reemplazo del molde: sin el, el servidor responde 400 en vez de pisar el
   * trabajo de otra persona.
   */
  ifMatch?: string
}

export interface ClienteApi {
  get<T>(ruta: string, init?: OpcionesPeticion): Promise<T>
  post<T>(ruta: string, cuerpo?: unknown, init?: OpcionesPeticion): Promise<T>
  /**
   * Un reemplazo lleva siempre su `ifMatch`, y el tipo lo exige para que no se
   * olvide. La excepcion es un `PUT` que fija un estado sin version —darle un
   * rol a una cuenta—: ahi no hay nada que pisar, y se dice con
   * `sinVersion: true` en vez de mandar una cabecera vacia.
   */
  put<T>(ruta: string, cuerpo: unknown, init: OpcionesEscritura & ({ ifMatch: string } | { sinVersion: true })): Promise<T>
  delete(ruta: string, init?: OpcionesPeticion): Promise<void>
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
 * El cliente del navegador, con las dos etapas de docs/07-frontend.md: el
 * refresh ante un 401 y la normalizacion de todo fallo a ApiError.
 *
 * El token CSRF se lee de la cookie EN CADA PETICION y no se guarda. El login y
 * el refresh lo rotan: una copia en memoria manda el viejo y responde 403 justo
 * despues de renovar.
 */
export function crearCliente({ base, fetch: pedir, cookies, candado = sinCandado }: OpcionesCliente): ClienteApi {
  async function tokenCSRF(signal?: AbortSignal): Promise<string | undefined> {
    const actual = leerCookie(COOKIE_CSRF, cookies())
    if (actual) return actual

    // Sin cookie, el primer POST de un visitante nuevo —el login— no tendria
    // nada que copiar y el servidor responderia 403 sin que la persona pueda
    // hacer nada. Pasa siempre en `/login`: la pagina la sirve Nuxt, no Go, asi
    // que abrirla no siembra la cookie.
    await enviarUnaVez('GET', RUTA_SEMILLA, undefined, { signal })
    return leerCookie(COOKIE_CSRF, cookies())
  }

  // La promesa del refresh en vuelo en ESTA pestana. Cinco peticiones que
  // reciben 401 a la vez esperan la misma: sin esto saldrian cinco refresh, el
  // primero rotaria el token y los otros cuatro presentarian uno ya usado, que
  // el servidor trata como robo.
  let renovando: Promise<void> | null = null

  function renovar(csrfAlSalir: string | undefined): Promise<void> {
    renovando ??= candado(CANDADO_REFRESH, async () => {
      // Con el candado en la mano, puede que otra pestana haya renovado
      // mientras esta esperaba. El refresh rota XSRF-TOKEN, asi que si la
      // cookie ya no es la que habia al mandar la peticion, el `rt` de la cookie
      // es otro y basta con reintentar. Renovar otra vez presentaria el `rt`
      // que la otra pestana acaba de gastar: robo, y fuera todas las sesiones.
      //
      // Solo si HABIA token al salir. Tras reiniciar el navegador la cookie
      // CSRF —que es de sesion— ya no esta, pero `has_session` si; el primer
      // GET siembra una nueva, y compararla con "nada" pareceria una
      // renovacion ajena: se saltaria el refresh y un `rt` valido acabaria en
      // el login.
      const actual = leerCookie(COOKIE_CSRF, cookies())
      if (csrfAlSalir !== undefined && actual !== csrfAlSalir) return

      await enviarUnaVez<void>('POST', RUTA_REFRESH, undefined)
    }).finally(() => {
      renovando = null
    })
    return renovando
  }

  async function enviar<T>(metodo: string, ruta: string, cuerpo: unknown, init: OpcionesEscritura = {}): Promise<T> {
    const csrfAlSalir = leerCookie(COOKIE_CSRF, cookies())
    try {
      return await enviarUnaVez<T>(metodo, ruta, cuerpo, init)
    } catch (fallo) {
      const renovable =
        fallo instanceof ApiError &&
        fallo.status === 401 &&
        !SIN_RENOVACION.has(ruta) &&
        // Sin pista no hubo sesion: el 401 es la respuesta correcta y un
        // refresh solo seria otro 401.
        leerCookie(COOKIE_PISTA, cookies()) !== undefined
      if (!renovable) throw fallo

      // Si el refresh falla, sale SU error: un 401 dice "no hay sesion" y una
      // caida dice "no se", que es lo que el guard necesita distinguir.
      await renovar(csrfAlSalir)

      // Una sola vez. Un segundo 401 con la sesion recien renovada no se
      // arregla renovando otra vez, y reintentar en bucle es un cliente que
      // martillea al servidor.
      return enviarUnaVez<T>(metodo, ruta, cuerpo, init)
    }
  }

  async function enviarUnaVez<T>(metodo: string, ruta: string, cuerpo: unknown, init: OpcionesEscritura = {}): Promise<T> {
    const { signal, ifMatch } = init
    const cabeceras: Record<string, string> = { Accept: 'application/json, application/problem+json' }
    if (ifMatch !== undefined) cabeceras['If-Match'] = ifMatch

    if (metodo !== 'GET') {
      const token = await tokenCSRF(signal)
      // Sin token no se inventa uno: se manda sin cabecera y el servidor
      // responde 403, que es un error con traceId. Un valor inventado daria el
      // mismo 403 y ademas mentiria en el log.
      if (token) cabeceras[CABECERA_CSRF] = token
    }
    // Un FormData —una subida de archivo— viaja tal cual y sin Content-Type:
    // lo escribe el navegador con el boundary del multipart. Ponerlo a mano lo
    // pierde, y el servidor no encuentra el campo.
    const esFormulario = cuerpo instanceof FormData
    if (cuerpo !== undefined && !esFormulario) cabeceras['Content-Type'] = 'application/json'

    let res: Response
    try {
      res = await pedir(base + ruta, {
        method: metodo,
        headers: cabeceras,
        body: cuerpo === undefined ? undefined : esFormulario ? cuerpo : JSON.stringify(cuerpo),
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

  // El reintento tras un refresh repite la peticion ENTERA, If-Match incluido:
  // la version que el cliente creia estar editando no cambia porque se haya
  // renovado la sesion.
  return {
    get: (ruta, init) => enviar('GET', ruta, undefined, init),
    post: (ruta, cuerpo, init) => enviar('POST', ruta, cuerpo, init),
    put: (ruta, cuerpo, init) => enviar('PUT', ruta, cuerpo, init),
    delete: (ruta, init) => enviar<void>('DELETE', ruta, undefined, init),
  }
}

const sinCandado: Candado = (_nombre, trabajo) => trabajo()

/**
 * El candado de la app: Web Locks, que coordina TODAS las pestanas del mismo
 * origen. El mismo `rt` solo existe dos veces en un cookie jar, asi que es aqui
 * —y no en el servidor— donde se ordena la carrera de CU-002 A1. El servidor
 * trata todo reuso como robo.
 */
export function candadoDelNavegador(): Candado {
  const locks = globalThis.navigator?.locks
  if (!locks) return sinCandado
  return async (nombre, trabajo) => {
    await locks.request(nombre, trabajo)
  }
}
