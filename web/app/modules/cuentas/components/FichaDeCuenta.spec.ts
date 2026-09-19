// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import FichaDeCuenta from './FichaDeCuenta.vue'

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

const ana = (extra: Partial<Schemas['Cuenta']> = {}): Schemas['Cuenta'] => ({
  id: 'u1',
  email: 'ana@casa.com',
  displayName: 'Ana',
  enabled: true,
  roles: ['staff'],
  version: 3,
  createdAt: '2026-09-18T10:00:00Z',
  updatedAt: '2026-09-18T10:00:00Z',
  updatedBy: null,
  ...extra,
})

const catalogo: Schemas['RolesPage'] = {
  content: [
    { key: 'admin', name: 'Administracion', permissions: [] },
    { key: 'staff', name: 'Equipo', permissions: [] },
    { key: 'superadmin', name: 'Superadministracion', permissions: [] },
  ],
  page: { number: 0, size: 100, totalElements: 3, totalPages: 1 },
}

const fallo = (status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ status, code: 'X', message: 'x', answered: true, unavailable: false, ...extra })

function montar(props: Partial<InstanceType<typeof FichaDeCuenta>['$props']> = {}) {
  const w = mount(FichaDeCuenta, {
    props: {
      cargar: vi.fn(async () => ana()),
      guardar: vi.fn(async (_v: number, c: Schemas['CuentaModificacion']) => ana({ ...c, version: 4 })),
      deshabilitar: vi.fn(async () => ana({ enabled: false, version: 4 })),
      habilitar: vi.fn(async () => ana({ enabled: true, version: 4 })),
      darRol: vi.fn(async () => {}),
      quitarRol: vi.fn(async () => {}),
      cargarRoles: async () => catalogo,
      puedeEscribir: true,
      puedeAsignar: true,
      esPropia: false,
      ...props,
    } as never,
    attachTo: document.body,
  })
  montados.push(w)
  return w
}

const boton = (w: ReturnType<typeof montar>, texto: string) => w.findAll('button').find((b) => b.text().includes(texto))
const botonExacto = (texto: string) => [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === texto)

async function cambiarNombre(w: ReturnType<typeof montar>, nombre: string) {
  await w.find('[data-campo="displayName"]').setValue(nombre)
}

describe('FichaDeCuenta', () => {
  it('muestra la cuenta, su acceso y una casilla por rol del catalogo', async () => {
    const w = montar()
    await flushPromises()

    expect(w.find('h1').text()).toBe('Ana')
    expect(w.find('[data-insignia="acceso"]').text()).toContain('Habilitada')
    const casillas = w.findAll('input[type="checkbox"]')
    expect(casillas.map((c) => c.attributes('data-rol'))).toEqual(['admin', 'staff', 'superadmin'])
    expect(casillas.map((c) => (c.element as HTMLInputElement).checked)).toEqual([false, true, false])
  })

  it('sin el catalogo muestra solo los roles que tiene', async () => {
    const w = montar({ cargarRoles: undefined })
    await flushPromises()
    expect(w.findAll('input[type="checkbox"]').map((c) => c.attributes('data-rol'))).toEqual(['staff'])
  })

  it('guardar manda la version leida y solo correo y nombre', async () => {
    const w = montar()
    await flushPromises()

    await cambiarNombre(w, 'Ana Maria')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    const guardar = w.props('guardar') as ReturnType<typeof vi.fn>
    expect(guardar).toHaveBeenCalledWith(3, { email: 'ana@casa.com', displayName: 'Ana Maria' })
    expect(w.find('h1').text()).toBe('Ana Maria')
  })

  // El mismo 409 puede ser dos cosas. Si al releer la version sigue igual,
  // nadie guardo en medio: el conflicto es el correo.
  it('un 409 con la version intacta es el correo de otra cuenta', async () => {
    const guardar = vi.fn(async () => {
      throw fallo(409)
    })
    const w = montar({ guardar })
    await flushPromises()

    await w.find('[data-campo="email"]').setValue('bea@casa.com')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Ya hay otra cuenta con este correo')
    expect(w.find('[data-campo="email"]').attributes('aria-invalid')).toBe('true')
    expect(w.find('[data-aviso="conflicto"]').exists()).toBe(false)
    // Lo escrito no se pierde.
    expect((w.find('[data-campo="email"]').element as HTMLInputElement).value).toBe('bea@casa.com')
  })

  it('un 409 con otra version es que alguien guardo en medio, y no pisa lo escrito', async () => {
    const cargar = vi.fn(async () => ana())
    const guardar = vi.fn(async () => {
      cargar.mockResolvedValue(ana({ version: 4, displayName: 'La de otra persona', updatedAt: '2026-09-18T12:30:00Z' }))
      throw fallo(409)
    })
    const w = montar({ cargar, guardar })
    await flushPromises()

    await cambiarNombre(w, 'La mia')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-aviso="conflicto"]').text()).toContain('2026-09-18 12:30')
    expect((w.find('[data-campo="displayName"]').element as HTMLInputElement).value).toBe('La mia')
  })

  it('deshabilitar pide confirmacion antes de hacer nada', async () => {
    const deshabilitar = vi.fn(async () => ana({ enabled: false, version: 4 }))
    const w = montar({ deshabilitar })
    await flushPromises()

    await boton(w, 'Deshabilitar cuenta')!.trigger('click')
    await flushPromises()
    expect(deshabilitar).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('se cierran todas sus sesiones')

    // El de la confirmacion se llama solo "Deshabilitar"; el de la ficha, "Deshabilitar cuenta".
    botonExacto('Deshabilitar')!.click()
    await flushPromises()

    expect(deshabilitar).toHaveBeenCalledOnce()
    expect(w.find('[data-insignia="acceso"]').text()).toContain('Deshabilitada')
    expect(boton(w, 'Habilitar cuenta')).toBeDefined()
  })

  it('en la cuenta propia la confirmacion avisa que te saca', async () => {
    const w = montar({ esPropia: true })
    await flushPromises()
    await boton(w, 'Deshabilitar cuenta')!.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('Es tu propia cuenta')
  })

  // El 409 del ultimo superadmin trae un mensaje que dice que hacer, y es lo que
  // se muestra. La casilla vuelve a su estado: el rol no se quito.
  it('quitarle superadmin al ultimo muestra el mensaje del servidor y la casilla sigue marcada', async () => {
    const quitarRol = vi.fn(async () => {
      throw fallo(409, { message: 'Es el ultimo superadmin habilitado. Dale el rol a alguien mas antes de quitarselo a este' })
    })
    const w = montar({ cargar: async () => ana({ roles: ['superadmin'] }), quitarRol })
    await flushPromises()

    const casilla = w.find('input[data-rol="superadmin"]')
    await casilla.setValue(false)
    await flushPromises()

    expect(quitarRol).toHaveBeenCalledWith('superadmin')
    expect(w.find('[data-aviso="error"]').text()).toContain('Dale el rol a alguien mas')
    expect((casilla.element as HTMLInputElement).checked).toBe(true)
  })

  it('marcar una casilla da el rol', async () => {
    const darRol = vi.fn(async () => {})
    const w = montar({ darRol })
    await flushPromises()

    await w.find('input[data-rol="admin"]').setValue(true)
    await flushPromises()

    expect(darRol).toHaveBeenCalledWith('admin')
    expect((w.find('input[data-rol="admin"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('sin identity.role.assign las casillas no se pueden usar', async () => {
    const w = montar({ puedeAsignar: false })
    await flushPromises()
    expect(w.findAll('input[type="checkbox"]').every((c) => (c.element as HTMLInputElement).disabled)).toBe(true)
    expect(w.text()).toContain('identity.role.assign')
  })

  it('sin identity.user.write es de solo lectura y no ofrece deshabilitar', async () => {
    const w = montar({ puedeEscribir: false })
    await flushPromises()
    expect(w.find('[data-aviso="solo-lectura"]').exists()).toBe(true)
    expect(boton(w, 'Deshabilitar cuenta')).toBeUndefined()
    expect(boton(w, 'Guardar')).toBeUndefined()
  })

  it('una cuenta que no existe lo dice y no ofrece reintentar', async () => {
    const w = montar({
      cargar: async () => {
        throw fallo(404)
      },
    })
    await flushPromises()
    expect(w.text()).toContain('Esta cuenta no existe')
    expect(boton(w, 'Reintentar')).toBeUndefined()
  })

  it('una caida ofrece reintentar y deja ver el traceId', async () => {
    const w = montar({
      cargar: async () => {
        throw fallo(503, { unavailable: true, traceId: 'traza-9' })
      },
    })
    await flushPromises()
    expect(w.text()).toContain('El servidor no responde')
    expect(w.text()).toContain('traza-9')
    expect(boton(w, 'Reintentar')).toBeDefined()
  })

  it('carga con esqueleto', () => {
    const w = montar({ cargar: () => new Promise<never>(() => {}) })
    expect(w.find('[data-estado="cargando"]').exists()).toBe(true)
  })
})
