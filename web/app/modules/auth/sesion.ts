import type { ClienteApi } from '~/shared/api/client'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'

// La logica de sesion del frontend, sin Nuxt: se prueba en node, en
// milisegundos. Lo que toca el router o el estado global vive en
// `composables/` y en `middleware/`, y es pegamento de pocas lineas.

export type Perfil = Schemas['Perfil']
export type Credenciales = Schemas['Credenciales']

export { COOKIE_PISTA } from '~/shared/api/client'

export const RUTA_LOGIN = '/login'
const DESTINO_POR_OMISION = '/admin'

/**
 * CU-001 visto desde el navegador: el login responde 204 sin cuerpo, y lo que
 * el frontend sabe de la persona sale de `me`. Es una sola fuente a proposito.
 */
export async function iniciarSesion(api: ClienteApi, credenciales: Credenciales): Promise<Perfil> {
  await api.post<void>('/auth/login', credenciales)
  return pedirPerfil(api)
}

export function pedirPerfil(api: ClienteApi): Promise<Perfil> {
  return api.get<Perfil>('/auth/me')
}

/**
 * Cierra la sesion en el servidor. Si falla, NO lanza: el servidor borra las
 * cookies aunque su base falle, y quien pulso "cerrar sesion" tiene que salir
 * igual. Lo que no se puede es dejarlo dentro creyendo que salio.
 */
export async function cerrarSesion(api: ClienteApi): Promise<void> {
  try {
    await api.post<void>('/auth/logout')
  } catch {
    // El traceId del fallo queda en el log del servidor.
  }
}

/**
 * Lo protegido es `/admin` y todo lo que cuelga de el.
 *
 * Por prefijo y no por lista: una pantalla nueva bajo `/admin` queda protegida
 * sin que nadie se acuerde. `/administracion` NO cuenta: comparar con
 * `startsWith('/admin')` a secas la meteria, y el dia que exista una ruta
 * publica con ese nombre pediria sesion sin motivo.
 */
export function esRutaProtegida(ruta: string): boolean {
  return ruta === DESTINO_POR_OMISION || ruta.startsWith(`${DESTINO_POR_OMISION}/`)
}

/**
 * A donde se va despues de entrar.
 *
 * `?next=` lo escribe cualquiera, asi que sin esto `/login?next=https://otro.sitio`
 * convierte el login en una redireccion abierta: un enlace con el dominio de
 * verdad que, despues de pedir la contrasena, deja a la persona en una copia
 * del sitio. Por eso solo se acepta una ruta protegida de ESTE origen.
 *
 * Se resuelve con URL y no con expresiones: `//otro.sitio` y `/\otro.sitio`
 * empiezan por `/` y el navegador los trata como otro origen.
 */
export function destinoSeguro(next: unknown): string {
  if (typeof next !== 'string' || next === '') return DESTINO_POR_OMISION

  const origen = 'http://origen.invalid'
  let url: URL
  try {
    url = new URL(next, origen)
  } catch {
    return DESTINO_POR_OMISION
  }

  if (url.origin !== origen || !esRutaProtegida(url.pathname)) return DESTINO_POR_OMISION
  return url.pathname + url.search + url.hash
}

/**
 * El texto que ve quien no pudo entrar.
 *
 * El 401 no usa el `detail` del servidor —"la peticion no trae una sesion
 * valida"— porque ese texto es de sesion, no de login. Y es UNO solo para
 * correo inexistente, contrasena incorrecta y cuenta deshabilitada: el servidor
 * ya responde identico en los tres casos, y un mensaje distinto aqui
 * reconstruiria en el cliente el oraculo que alli se cerro.
 *
 * Devuelve `undefined` para una cancelacion: la pidio la propia pantalla.
 */
export function mensajeDeLogin(error: ApiError): string | undefined {
  if (!error.answered && !error.unavailable) return undefined
  if (error.unavailable) return 'El servidor no responde. Vuelve a intentarlo en un momento.'

  switch (error.status) {
    case 401:
      return 'Correo o contrasena incorrectos.'
    case 429: {
      if (error.retryAfter === undefined) return 'Demasiados intentos. Vuelve a probar en unos minutos.'
      const minutos = Math.max(1, Math.ceil(error.retryAfter / 60))
      return `Demasiados intentos. Vuelve a probar en ${minutos} minuto(s).`
    }
    case 403:
      // En el login, un 403 es el CSRF: la cookie cambio en otra pestana o la
      // borro alguien. Recargar la vuelve a sembrar.
      return 'No se pudo verificar la peticion. Recarga la pagina y vuelve a intentarlo.'
    default:
      return 'Algo fallo al iniciar sesion. Si se repite, comparte la referencia de abajo.'
  }
}

export type Acceso =
  | { tipo: 'pasar'; perfil: Perfil }
  | { tipo: 'login' }
  | { tipo: 'error'; error: ApiError }

/**
 * Que hacer con una navegacion a una ruta protegida.
 *
 * Tres salidas y ninguna cuarta, que es la postcondicion de CU-003:
 *
 * - **pasar** si ya hay perfil, o si `me` lo devuelve
 * - **login** si no hay pista de sesion —sin pedir nada— o si `me` responde 401
 * - **error** para todo lo demas. Una caida NO manda al login: expulsar a
 *   alguien porque el servidor se cayo le hace perder lo que estaba haciendo, y
 *   al volver el servidor seguiria teniendo sesion
 *
 * El `at` caducado no llega aqui como 401: el cliente renueva y reintenta `me`
 * antes de devolver nada (CU-002). Un 401 que SI llega es que no habia sesion
 * que renovar.
 */
export async function decidirAcceso(entrada: {
  perfil: Perfil | null
  pista: boolean
  pedirPerfil: () => Promise<Perfil>
}): Promise<Acceso> {
  if (entrada.perfil) return { tipo: 'pasar', perfil: entrada.perfil }
  if (!entrada.pista) return { tipo: 'login' }

  try {
    return { tipo: 'pasar', perfil: await entrada.pedirPerfil() }
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 401) return { tipo: 'login' }
    return { tipo: 'error', error: causa }
  }
}
