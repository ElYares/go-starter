import type { ClienteApi } from '~/shared/api/client'
import type { Schemas } from '~/shared/api/generated'

// Las operaciones de content que usa el dashboard, sobre el cliente del
// navegador. Sin Nuxt: se prueban con un cliente falso.

export type Pagina = Schemas['Pagina']

/** El ETag del molde es la version entre comillas (httpx.ETag en el api). */
export function ifMatchDe(version: number): string {
  return `"${version}"`
}

export const listarPaginas = (api: ClienteApi) =>
  // El tope del molde. Una landing con mas de cien paginas no es el caso del starter.
  api.get<Schemas['PaginasPage']>('/pages?size=100&sort=updatedAt,desc')

export const crearPagina = (api: ClienteApi, nueva: Schemas['PaginaNueva']) => api.post<Pagina>('/pages', nueva)

export const leerPagina = (api: ClienteApi, id: string) => api.get<Pagina>(`/pages/${id}`)

/** Guarda sobre la version que se leyo. Si otra persona guardo en medio, 409. */
export const guardarPagina = (api: ClienteApi, id: string, version: number, cuerpo: Schemas['PaginaModificacion']) =>
  api.put<Pagina>(`/pages/${id}`, cuerpo, { ifMatch: ifMatchDe(version) })

export const publicarVersion = (api: ClienteApi, id: string, versionId: string) =>
  api.post<Pagina>(`/pages/${id}/publish`, { versionId } satisfies Schemas['Publicacion'])

export const borrarPagina = (api: ClienteApi, id: string) => api.delete(`/pages/${id}`)

export const listarVersiones = (api: ClienteApi, id: string) =>
  api.get<Schemas['VersionesPage']>(`/pages/${id}/versions?size=100`)
