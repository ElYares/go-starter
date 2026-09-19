import type { Schemas } from '~/shared/api/generated'
import type { ApiDeSesion, Perfil } from './sesion'

// La contrasena vista desde el navegador (HU-019): pedir una temporal sin
// sesion, cambiar la propia, y a donde se obliga a ir con una temporal. Sin
// Nuxt: se prueba con un cliente falso.

export const RUTA_CAMBIAR_CONTRASENA = '/admin/contrasena'
export const RUTA_PEDIR_CONTRASENA = '/recuperar-contrasena'

/** Responde lo mismo exista o no la cuenta: el mensaje lo pone el api. */
export const pedirContrasena = (api: ApiDeSesion, email: string) =>
  api.post<Schemas['PedidoRecibido']>('/auth/password-reset', { email } satisfies Schemas['PedidoDeContrasena'])

/** El api revoca las demas sesiones y emite cookies nuevas para esta. */
export const cambiarContrasena = (api: ApiDeSesion, cuerpo: Schemas['CambioDeContrasena']) =>
  api.post<void>('/auth/password', cuerpo)

/**
 * A donde tiene que ir alguien antes que a donde pidio, o `null` si puede
 * seguir.
 *
 * Con una contrasena temporal el api no le resuelve ningun permiso, asi que
 * cualquier pantalla del dashboard le responderia 403. Mandarla a cambiarla es
 * lo unico util que puede hacer; el api lo impone igual si alguien se salta
 * esto.
 */
export function destinoObligado(perfil: Perfil, ruta: string): string | null {
  return perfil.mustChangePassword && ruta !== RUTA_CAMBIAR_CONTRASENA ? RUTA_CAMBIAR_CONTRASENA : null
}
