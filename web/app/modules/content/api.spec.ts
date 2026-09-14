import { describe, expect, it, vi } from 'vitest'
import type { ClienteApi } from '~/shared/api/client'
import { borrarPagina, guardarPagina, ifMatchDe, listarVersiones, publicarVersion } from './api'

function clienteFalso(): ClienteApi {
  return {
    get: vi.fn(async () => ({}) as never),
    post: vi.fn(async () => ({}) as never),
    put: vi.fn(async () => ({}) as never),
    delete: vi.fn(async () => undefined),
  }
}

describe('las operaciones de content', () => {
  // El api compara la version del If-Match con la de la base. Un ETag sin
  // comillas tambien lo acepta, pero el molde lo escribe con ellas.
  it('guardar manda la version leida como If-Match', async () => {
    const api = clienteFalso()
    const cuerpo = { slug: 's', title: 'T', blocks: [] }

    await guardarPagina(api, 'p1', 7, cuerpo)

    expect(ifMatchDe(7)).toBe('"7"')
    expect(api.put).toHaveBeenCalledWith('/pages/p1', cuerpo, { ifMatch: '"7"' })
  })

  it('publicar manda solo el versionId', async () => {
    const api = clienteFalso()
    await publicarVersion(api, 'p1', 'v9')
    expect(api.post).toHaveBeenCalledWith('/pages/p1/publish', { versionId: 'v9' })
  })

  it('borrar y listar versiones van a la pagina pedida', async () => {
    const api = clienteFalso()
    await borrarPagina(api, 'p1')
    await listarVersiones(api, 'p1')
    expect(api.delete).toHaveBeenCalledWith('/pages/p1')
    expect(api.get).toHaveBeenCalledWith('/pages/p1/versions?size=100')
  })
})
