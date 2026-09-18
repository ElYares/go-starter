import type { ClienteApi } from '~/shared/api/client'
import type { Schemas } from '~/shared/api/generated'

// Las operaciones de settings que usa el dashboard, sobre el cliente del
// navegador. Sin Nuxt: se prueban con un cliente falso.

export type Setting = Schemas['Setting']

export const listarSettings = (api: ClienteApi) =>
  // El tope del molde. `page.totalElements` dice si faltan.
  api.get<Schemas['SettingsPage']>('/settings?size=100')

export const leerSetting = (api: ClienteApi, clave: string) =>
  api.get<Setting>(`/settings/${encodeURIComponent(clave)}`)

/**
 * Guarda el valor sobre la version que se leyo. No manda `isPublic`: ausente
 * conserva la visibilidad (Decision 025), y esta pantalla no la cambia.
 */
export const guardarSetting = (api: ClienteApi, clave: string, version: number, valor: unknown) =>
  api.put<Setting>(`/settings/${encodeURIComponent(clave)}`, { value: valor } satisfies Schemas['SettingModificacion'], {
    // El ETag del molde es la version entre comillas (httpx.ETag en el api).
    ifMatch: `"${version}"`,
  })

/** Sube una imagen. Los mismos bytes dos veces devuelven el mismo registro. */
export function subirMedio(api: ClienteApi, archivo: File) {
  const form = new FormData()
  form.append('file', archivo)
  return api.post<Schemas['Medio']>('/media', form)
}
