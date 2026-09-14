import { ApiError } from '~/shared/api/errors'
import { RUTA_LOGIN } from '../sesion'
import { useSesion } from './useSesion'

/**
 * Envuelve una peticion de una pantalla del dashboard para que perder la sesion
 * a mitad de uso lleve al login (CU-002 E1).
 *
 * El cliente ya renovo y reintento antes de que llegue aqui un 401: si llega,
 * no hay sesion que renovar. El guard solo lo detecta al navegar; una pantalla
 * que ya esta abierta y pide datos tiene que hacerlo ella.
 *
 * Devuelve una promesa que no se resuelve nunca en ese caso, a proposito: la
 * pantalla se queda en "cargando" el instante que tarda en irse, en vez de
 * pintar un error que la persona leeria justo antes de ver el login.
 */
export function usePedirConSesion() {
  const { perfil } = useSesion()
  const route = useRoute()

  return async function pedir<T>(peticion: () => Promise<T>): Promise<T> {
    try {
      return await peticion()
    } catch (fallo) {
      if (fallo instanceof ApiError && fallo.status === 401) {
        perfil.value = null
        await navigateTo({ path: RUTA_LOGIN, query: { next: route.fullPath } }, { replace: true })
        return new Promise<T>(() => {})
      }
      throw fallo
    }
  }
}
