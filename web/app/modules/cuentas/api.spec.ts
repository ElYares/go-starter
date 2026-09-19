import { describe, expect, it, vi } from 'vitest'
import type { ClienteApi } from '~/shared/api/client'
import { darRol, deshabilitarCuenta, guardarCuenta, habilitarCuenta, listarCuentas, quitarRol } from './api'

function clienteFalso(): ClienteApi {
  return {
    get: vi.fn(async () => ({}) as never),
    post: vi.fn(async () => ({}) as never),
    put: vi.fn(async () => ({}) as never),
    delete: vi.fn(async () => undefined),
  }
}

describe('las operaciones de cuentas', () => {
  it('listar pide el tope del molde, ordenado por correo', async () => {
    const api = clienteFalso()
    await listarCuentas(api)
    expect(api.get).toHaveBeenCalledWith('/users?size=100&sort=email')
  })

  it('guardar manda la version leida como If-Match', async () => {
    const api = clienteFalso()
    const cuerpo = { email: 'ana@casa.com', displayName: 'Ana' }
    await guardarCuenta(api, 'u1', 4, cuerpo)
    expect(api.put).toHaveBeenCalledWith('/users/u1', cuerpo, { ifMatch: '"4"' })
  })

  it('habilitar y deshabilitar son acciones sin cuerpo', async () => {
    const api = clienteFalso()
    await deshabilitarCuenta(api, 'u1')
    await habilitarCuenta(api, 'u1')
    expect(api.post).toHaveBeenCalledWith('/users/u1/disable')
    expect(api.post).toHaveBeenCalledWith('/users/u1/enable')
  })

  // Darle un rol no reemplaza nada con version: no manda If-Match, ni vacio.
  it('dar y quitar un rol van a su URL, con la clave codificada', async () => {
    const api = clienteFalso()
    await darRol(api, 'u1', 'jefe de sala')
    await quitarRol(api, 'u1', 'staff')
    expect(api.put).toHaveBeenCalledWith('/users/u1/roles/jefe%20de%20sala', undefined, { sinVersion: true })
    expect(api.delete).toHaveBeenCalledWith('/users/u1/roles/staff')
  })
})
