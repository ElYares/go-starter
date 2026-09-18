import { describe, expect, it, vi } from 'vitest'
import type { ClienteApi } from '~/shared/api/client'
import { guardarSetting, leerSetting, subirMedio } from './api'

function clienteFalso() {
  return {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
    put: vi.fn().mockResolvedValue({}),
    delete: vi.fn().mockResolvedValue(undefined),
  } satisfies ClienteApi
}

describe('las operaciones de settings', () => {
  it('guarda con If-Match y sin isPublic, para no tocar la visibilidad', async () => {
    const api = clienteFalso()
    await guardarSetting(api, 'site.brand', 7, { name: 'X' })
    expect(api.put).toHaveBeenCalledWith('/settings/site.brand', { value: { name: 'X' } }, { ifMatch: '"7"' })
  })

  it('lee por clave', async () => {
    const api = clienteFalso()
    await leerSetting(api, 'site.nav')
    expect(api.get).toHaveBeenCalledWith('/settings/site.nav')
  })

  it('sube la imagen como multipart en el campo file', async () => {
    const api = clienteFalso()
    const archivo = new File(['x'], 'logo.png', { type: 'image/png' })
    await subirMedio(api, archivo)
    const [ruta, cuerpo] = api.post.mock.calls[0]!
    expect(ruta).toBe('/media')
    expect(cuerpo).toBeInstanceOf(FormData)
    expect((cuerpo as FormData).get('file')).toBe(archivo)
  })
})
