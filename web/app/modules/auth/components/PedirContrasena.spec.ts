// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import PedirContrasena from './PedirContrasena.vue'

const MENSAJE = 'Si la cuenta existe, quien administra el sitio vera tu solicitud.'
const fallo = (status: number, extra = {}) =>
  new ApiError({ status, code: 'X', message: 'x', answered: true, unavailable: false, ...extra })

async function enviar(pedir: (email: string) => Promise<{ message: string }>, email = 'ana@casa.com') {
  const w = mount(PedirContrasena, { props: { pedir } })
  await w.find('input').setValue(email)
  await w.find('form').trigger('submit')
  await flushPromises()
  return w
}

describe('PedirContrasena', () => {
  // El texto es el del api, igual exista o no la cuenta. Aqui no se inventa
  // otro, porque uno distinto por caso delataria lo que el api calla.
  it('muestra el mensaje del api, tal cual', async () => {
    const pedir = vi.fn(async () => ({ message: MENSAJE }))
    const w = await enviar(pedir)
    expect(pedir).toHaveBeenCalledWith('ana@casa.com')
    expect(w.find('[data-estado="recibido"]').text()).toContain(MENSAJE)
  })

  it('sin correo no manda nada', async () => {
    const pedir = vi.fn()
    const w = await enviar(pedir, '  ')
    expect(pedir).not.toHaveBeenCalled()
    expect(w.text()).toContain('Escribe el correo')
  })

  it('un 400 marca el correo', async () => {
    const w = await enviar(async () => {
      throw fallo(400, { errors: [{ field: 'email', code: 'format', message: 'Tiene que ser del estilo nombre@dominio.com' }] })
    })
    expect(w.text()).toContain('nombre@dominio.com')
    expect(w.find('input').attributes('aria-invalid')).toBe('true')
  })

  it('un 429 pide esperar', async () => {
    const w = await enviar(async () => {
      throw fallo(429)
    })
    expect(w.find('[role="alert"]').text()).toContain('Espera un minuto')
  })
})
