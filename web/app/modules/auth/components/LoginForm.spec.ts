// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import LoginForm from './LoginForm.vue'

const montados: Array<{ unmount: () => void }> = []

afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(entrar: (c: { email: string; password: string }) => Promise<void>) {
  const w = mount(LoginForm, { props: { entrar }, attachTo: document.body })
  montados.push(w)
  return w
}

function error(status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) {
  return new ApiError({ status, code: 'X', message: 'x', answered: true, unavailable: false, ...extra })
}

async function llenarYEnviar(w: ReturnType<typeof montar>, email = 'ana@casa.com', password = 'una-contrasena-larga') {
  await w.find('input[type="email"]').setValue(email)
  await w.find('input[type="password"]').setValue(password)
  await w.find('form').trigger('submit')
}

describe('LoginForm', () => {
  it('manda el correo sin pasarlo a minusculas y la contrasena tal cual', async () => {
    const entrar = vi.fn(async () => {})
    const w = montar(entrar)

    await llenarYEnviar(w, 'Ana@Casa.com', ' con espacios ')
    await flushPromises()

    // Sin minusculas: la igualdad la resuelve el citext de la base. Y la
    // contrasena sin recortar: un espacio al final es parte de ella.
    expect(entrar).toHaveBeenCalledWith({ email: 'Ana@Casa.com', password: ' con espacios ' })
  })

  it('con campos vacios no llama al servidor y marca los dos', async () => {
    const entrar = vi.fn(async () => {})
    const w = montar(entrar)

    await w.find('form').trigger('submit')

    expect(entrar).not.toHaveBeenCalled()
    expect(w.findAll('input[aria-invalid="true"]')).toHaveLength(2)
  })

  it('no marca errores antes del primer envio', () => {
    const w = montar(vi.fn())

    expect(w.findAll('[role="alert"]')).toHaveLength(0)
  })

  // Un segundo clic mientras el primero viaja son dos intentos contra el limite.
  it('no admite un segundo envio mientras el primero esta en vuelo', async () => {
    let soltar!: () => void
    const entrar = vi.fn(() => new Promise<void>((r) => (soltar = r)))
    const w = montar(entrar)

    await llenarYEnviar(w)
    await w.find('form').trigger('submit')

    expect(entrar).toHaveBeenCalledTimes(1)
    expect(w.find('button').attributes('aria-busy')).toBe('true')

    soltar()
    await flushPromises()
    expect(w.find('button').attributes('aria-busy')).toBeUndefined()
  })

  it('un 401 se anuncia con el mensaje generico, borra la contrasena y conserva el correo', async () => {
    const w = montar(async () => {
      throw error(401, { traceId: 't-401' })
    })

    await llenarYEnviar(w)
    await flushPromises()

    const alerta = w.find('.fallo[role="alert"]')
    expect(alerta.text()).toContain('Correo o contrasena incorrectos.')
    // En un 401 la referencia es ruido: la persona ya sabe que paso.
    expect(alerta.text()).not.toContain('t-401')
    expect((w.find('input[type="password"]').element as HTMLInputElement).value).toBe('')
    expect((w.find('input[type="email"]').element as HTMLInputElement).value).toBe('ana@casa.com')
  })

  // Borrar la contrasena tras el fallo no puede dejar el campo marcado como
  // "obligatorio": la persona no hizo nada mal todavia.
  it('tras un fallo, el campo vaciado no aparece como invalido', async () => {
    const w = montar(async () => {
      throw error(401)
    })

    await llenarYEnviar(w)
    await flushPromises()

    expect(w.findAll('input[aria-invalid="true"]')).toHaveLength(0)
  })

  it('un 500 ensena la referencia para soporte', async () => {
    const w = montar(async () => {
      throw error(500, { traceId: 'abc123' })
    })

    await llenarYEnviar(w)
    await flushPromises()

    expect(w.find('.fallo').text()).toContain('abc123')
  })

  it('un 429 dice cuanto esperar', async () => {
    const w = montar(async () => {
      throw error(429, { retryAfter: 900 })
    })

    await llenarYEnviar(w)
    await flushPromises()

    expect(w.find('.fallo').text()).toContain('15 minuto(s)')
  })

  it('un nuevo envio limpia el fallo anterior', async () => {
    let veces = 0
    let soltar!: () => void
    const w = montar(() => {
      veces++
      if (veces === 1) return Promise.reject(error(401))
      return new Promise<void>((r) => (soltar = r))
    })

    await llenarYEnviar(w)
    await flushPromises()
    expect(w.find('.fallo').exists()).toBe(true)

    await llenarYEnviar(w)
    expect(w.find('.fallo').exists()).toBe(false)
    soltar()
  })

  // Lo que no es de la API es un bug, y un bug escondido detras de "algo
  // fallo" no lo encuentra nadie.
  it('un error que no es ApiError no se convierte en mensaje', async () => {
    const w = montar(async () => {
      throw new RangeError('bug')
    })
    const errores: unknown[] = []
    w.vm.$.appContext.config.errorHandler = (e) => {
      errores.push(e)
    }

    await llenarYEnviar(w)
    await flushPromises()

    expect(w.find('.fallo').exists()).toBe(false)
    // Y sube: sin esto, tragarse el error tambien dejaria la pantalla sin mensaje
    // y la prueba pasaria igual.
    expect(errores).toHaveLength(1)
    expect(errores[0]).toBeInstanceOf(RangeError)
    // El boton no se queda girando para siempre.
    expect(w.find('button').attributes('aria-busy')).toBeUndefined()
  })

  it('los campos llevan el autocompletado que usan los gestores de contrasenas', () => {
    const w = montar(vi.fn())

    expect(w.find('input[type="email"]').attributes('autocomplete')).toBe('username')
    expect(w.find('input[type="password"]').attributes('autocomplete')).toBe('current-password')
  })
})
