import type { ClienteApi } from '~/shared/api/client'
import type { Schemas } from '~/shared/api/generated'
import { ifMatchDe } from '~/modules/content/api'

// Las operaciones del dashboard de cuentas (HU-018), sobre el cliente del
// navegador. Sin Nuxt: se prueban con un cliente falso.

export type Cuenta = Schemas['Cuenta']
export type Rol = Schemas['Rol']

export const listarCuentas = (api: ClienteApi) =>
  // El tope del molde. Un equipo de mas de cien cuentas no es el caso del starter.
  api.get<Schemas['CuentasPage']>('/users?size=100&sort=email')

export const crearCuenta = (api: ClienteApi, nueva: Schemas['CuentaNueva']) => api.post<Cuenta>('/users', nueva)

export const leerCuenta = (api: ClienteApi, id: string) => api.get<Cuenta>(`/users/${id}`)

/** Guarda sobre la version que se leyo. Si otra persona guardo en medio, 409. */
export const guardarCuenta = (api: ClienteApi, id: string, version: number, cuerpo: Schemas['CuentaModificacion']) =>
  api.put<Cuenta>(`/users/${id}`, cuerpo, { ifMatch: ifMatchDe(version) })

export const deshabilitarCuenta = (api: ClienteApi, id: string) => api.post<Cuenta>(`/users/${id}/disable`)

export const habilitarCuenta = (api: ClienteApi, id: string) => api.post<Cuenta>(`/users/${id}/enable`)

// El rol va en la URL y se codifica: la clave la escribe un fork. Es un PUT sin
// version: darle a alguien un rol que ya tiene es el mismo estado.
export const darRol = (api: ClienteApi, id: string, rol: string) =>
  api.put<void>(`/users/${id}/roles/${encodeURIComponent(rol)}`, undefined, { sinVersion: true })

export const quitarRol = (api: ClienteApi, id: string, rol: string) =>
  api.delete(`/users/${id}/roles/${encodeURIComponent(rol)}`)

export const listarRoles = (api: ClienteApi) => api.get<Schemas['RolesPage']>('/roles?size=100')

/** Una contrasena temporal: la cuenta tendra que cambiarla al entrar (HU-019). */
export const asignarContrasena = (api: ClienteApi, id: string, password: string) =>
  api.post<void>(`/users/${id}/password`, { password } satisfies Schemas['ContrasenaTemporal'])

export const listarSolicitudes = (api: ClienteApi) => api.get<Schemas['SolicitudesPage']>('/password-resets?size=100')
