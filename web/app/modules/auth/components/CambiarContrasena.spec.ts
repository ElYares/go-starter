// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import CambiarContrasena from './CambiarContrasena.vue'

function montar(cambiar: (c: { currentPassword: string; newPassword: string }) => Promise<void>, temporal = false) {
  return mount(CambiarContrasena, { props: { cambiar, temporal } })
}

async function llenarYEnviar(w: ReturnType<typeof montar>, actual: string, nueva: string) {
  await w.find('[data-campo="currentPassword"] input').setValue(actual)
  await w.find('[data-campo="newPassword"] input').setValue(nueva)
  await w.find('form').trigger('submit')
  await flushPromises()
}

describe('CambiarContrasena', () => {
  it('manda la actual y la nueva, y avisa', async () => {
    const cambiar = vi.fn(async () => {})
    const w = montar(cambiar)
    await llenarYEnviar(w, 'la de antes larga', 'la nueva bien larga')
    expect(cambiar).toHaveBeenCalledWith({ currentPassword: 'la de antes larga', newPassword: 'la nueva bien larga' })
    expect(w.emitted('cambiada')).toHaveLength(1)
  })

  it('con una temporal lo dice y llama "temporal" a la actual', () => {
    const w = montar(async () => {}, true)
    expect(w.find('[data-aviso="temporal"]').exists()).toBe(true)
    expect(w.text()).toContain('Contrasena temporal')
  })

  // La actual se escribe, no se genera; la nueva si.
  it('solo la nueva ofrece generar', () => {
    const w = montar(async () => {})
    expect(w.find('[data-campo="currentPassword"] [data-accion="generar"]').exists()).toBe(false)
    expect(w.find('[data-campo="newPassword"] [data-accion="generar"]').exists()).toBe(true)
  })

  it('un 400 marca el campo que nombra el api y no avisa de exito', async () => {
    const w = montar(async () => {
      throw new ApiError({
        status: 400, code: 'VALIDATION_FAILED', message: 'x', answered: true, unavailable: false,
        errors: [{ field: 'currentPassword', code: 'mismatch', message: 'No es tu contrasena actual' }],
      })
    })
    await llenarYEnviar(w, 'no es la mia', 'la nueva bien larga')
    expect(w.text()).toContain('No es tu contrasena actual')
    expect(w.emitted('cambiada')).toBeUndefined()
  })

  it('sin llenar los dos campos no se puede enviar', async () => {
    const cambiar = vi.fn()
    const w = montar(cambiar)
    await w.find('form').trigger('submit')
    expect(cambiar).not.toHaveBeenCalled()
  })
})
