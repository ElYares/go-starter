import { describe, expect, it, vi } from 'vitest'
import { cambiarContrasena, destinoObligado, pedirContrasena, RUTA_CAMBIAR_CONTRASENA } from './contrasena'
import type { Perfil } from './sesion'

const perfil = (mustChangePassword: boolean): Perfil => ({
  id: 'u1',
  email: 'ana@casa.com',
  displayName: 'Ana',
  roles: ['staff'],
  permissions: [],
  mustChangePassword,
})

describe('destinoObligado', () => {
  it('con contrasena temporal, todo lleva a cambiarla', () => {
    for (const ruta of ['/admin', '/admin/paginas', '/admin/usuarios/x']) {
      expect(destinoObligado(perfil(true), ruta)).toBe(RUTA_CAMBIAR_CONTRASENA)
    }
  })

  // Sin esto, la pantalla de cambiarla se redirigiria a si misma para siempre.
  it('en la pantalla de cambiarla, deja quedarse', () => {
    expect(destinoObligado(perfil(true), RUTA_CAMBIAR_CONTRASENA)).toBeNull()
  })

  it('sin temporal no obliga a nada', () => {
    expect(destinoObligado(perfil(false), '/admin/paginas')).toBeNull()
  })
})

describe('las llamadas', () => {
  it('pedir manda solo el correo, y cambiar la actual y la nueva', async () => {
    const api = { get: vi.fn(), post: vi.fn(async () => ({}) as never) }
    await pedirContrasena(api, 'ana@casa.com')
    await cambiarContrasena(api, { currentPassword: 'a', newPassword: 'b' })
    expect(api.post).toHaveBeenCalledWith('/auth/password-reset', { email: 'ana@casa.com' })
    expect(api.post).toHaveBeenCalledWith('/auth/password', { currentPassword: 'a', newPassword: 'b' })
  })
})
