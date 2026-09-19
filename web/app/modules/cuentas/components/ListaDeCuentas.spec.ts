// @vitest-environment happy-dom
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import ListaDeCuentas from './ListaDeCuentas.vue'

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

const cuenta = (email: string, extra: Partial<Schemas['Cuenta']> = {}): Schemas['Cuenta'] => ({
  id: `id-${email}`,
  email,
  displayName: email.split('@')[0]!.toUpperCase(),
  enabled: true,
  roles: [],
  version: 1,
  createdAt: '2026-09-18T10:00:00Z',
  updatedAt: '2026-09-18T10:00:00Z',
  updatedBy: null,
  ...extra,
})

const pagina = (content: Schemas['Cuenta'][]): Schemas['CuentasPage'] => ({
  content,
  page: { number: 0, size: 100, totalElements: content.length, totalPages: 1 },
})

const fallo = (status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ status, code: 'X', message: 'x', answered: true, unavailable: false, ...extra })

function montar(props: Partial<InstanceType<typeof ListaDeCuentas>['$props']> = {}) {
  const w = mount(ListaDeCuentas, {
    props: {
      cargar: async () =>
        pagina([
          cuenta('ana@casa.com', { roles: ['admin', 'staff'] }),
          cuenta('bea@casa.com', { enabled: false }),
        ]),
      crear: vi.fn(),
      puedeEscribir: true,
      ...props,
    } as never,
    attachTo: document.body,
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
  montados.push(w)
  return w
}

const enDocumento = (selector: string) => document.querySelector(selector) as HTMLInputElement | null
const botonEnDocumento = (texto: string) => [...document.querySelectorAll('button')].find((b) => b.textContent?.includes(texto))

function escribir(selector: string, valor: string) {
  const campo = enDocumento(selector)!
  campo.value = valor
  campo.dispatchEvent(new Event('input'))
}

async function abrirAlta(w: ReturnType<typeof montar>) {
  await flushPromises()
  await w.findAll('button').find((b) => b.text().includes('Nueva cuenta'))!.trigger('click')
  await flushPromises()
}

describe('ListaDeCuentas', () => {
  it('lista cada cuenta con su enlace a la ficha, sus roles y si puede entrar', async () => {
    const w = montar()
    await flushPromises()

    expect(w.findAllComponents(RouterLinkStub).map((l) => l.props('to'))).toEqual([
      '/admin/usuarios/id-ana@casa.com',
      '/admin/usuarios/id-bea@casa.com',
    ])
    expect(w.text()).toContain('admin')
    expect(w.text()).toContain('staff')
    expect(w.text()).toContain('Sin roles')
    expect(w.text()).toContain('Habilitada')
    expect(w.text()).toContain('Deshabilitada')
  })

  it('vacio invita a crear la primera, si se puede', async () => {
    const w = montar({ cargar: async () => pagina([]) })
    await flushPromises()
    expect(w.text()).toContain('Todavia no hay cuentas')
    expect(w.findAll('button').some((b) => b.text().includes('Crear la primera cuenta'))).toBe(true)
  })

  it('un error ofrece reintentar y deja ver el traceId', async () => {
    const cargar = vi.fn(async () => {
      throw fallo(500, { traceId: 'traza-1' })
    })
    const w = montar({ cargar })
    await flushPromises()

    expect(w.text()).toContain('No se pudieron cargar las cuentas')
    expect(w.text()).toContain('traza-1')
    await w.findAll('button').find((b) => b.text().includes('Reintentar'))!.trigger('click')
    expect(cargar).toHaveBeenCalledTimes(2)
  })

  it('un 403 dice que falta identity.user.read y no ofrece reintentar', async () => {
    const w = montar({
      cargar: async () => {
        throw fallo(403)
      },
    })
    await flushPromises()
    expect(w.text()).toContain('identity.user.read')
    expect(w.findAll('button').some((b) => b.text().includes('Reintentar'))).toBe(false)
  })

  it('sin identity.user.write no ofrece crear', async () => {
    const w = montar({ puedeEscribir: false })
    await flushPromises()
    expect(w.findAll('button').some((b) => b.text().includes('Nueva cuenta'))).toBe(false)
  })

  it('el alta manda correo, nombre y contrasena, sin roles, y avisa', async () => {
    const creada = cuenta('cris@casa.com')
    const crear = vi.fn(async () => creada)
    const w = montar({ crear })
    await abrirAlta(w)

    escribir('[data-campo="displayName"]', 'Cris')
    escribir('[data-campo="email"]', 'cris@casa.com')
    escribir('[data-campo="password"]', 'una clave larga de verdad')
    await flushPromises()
    expect(enDocumento('[data-campo="password"]')!.type).toBe('password')

    botonEnDocumento('Crear cuenta')!.click()
    await flushPromises()

    expect(crear).toHaveBeenCalledWith({ email: 'cris@casa.com', displayName: 'Cris', password: 'una clave larga de verdad' })
    expect(w.emitted('creada')).toEqual([[creada]])
  })

  it('un 400 marca el campo que nombra el servidor y el dialogo sigue abierto', async () => {
    const crear = vi.fn(async () => {
      throw fallo(400, { errors: [{ field: 'password', code: 'min', message: 'El minimo es 12 caracteres' }] })
    })
    const w = montar({ crear })
    await abrirAlta(w)

    botonEnDocumento('Crear cuenta')!.click()
    await flushPromises()

    expect(document.body.textContent).toContain('El minimo es 12 caracteres')
    expect(enDocumento('[data-campo="password"]')!.getAttribute('aria-invalid')).toBe('true')
    expect(w.emitted('creada')).toBeUndefined()
  })

  it('un correo ya usado se marca en su campo', async () => {
    const crear = vi.fn(async () => {
      throw fallo(409)
    })
    const w = montar({ crear })
    await abrirAlta(w)

    botonEnDocumento('Crear cuenta')!.click()
    await flushPromises()

    expect(document.body.textContent).toContain('Ya hay una cuenta con este correo')
    expect(enDocumento('[data-campo="email"]')!.getAttribute('aria-invalid')).toBe('true')
  })
})
