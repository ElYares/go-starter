// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import ListaDeSolicitudes from './ListaDeSolicitudes.vue'

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

const pagina = (content: Schemas['Solicitud'][]): Schemas['SolicitudesPage'] => ({
  content,
  page: { number: 0, size: 100, totalElements: content.length, totalPages: 1 },
})
const solicitud: Schemas['Solicitud'] = {
  id: 's1', userId: 'u1', email: 'ana@casa.com', displayName: 'Ana', createdAt: '2026-09-18T10:00:00Z',
}
const fallo = (status: number, extra = {}) =>
  new ApiError({ status, code: 'X', message: 'x', answered: true, unavailable: false, ...extra })

function montar(props: Record<string, unknown> = {}) {
  const w = mount(ListaDeSolicitudes, {
    props: { cargar: async () => pagina([solicitud]), asignar: vi.fn(async () => {}), ...props } as never,
    attachTo: document.body,
  })
  montados.push(w)
  return w
}
const botonEnDocumento = (texto: string) => [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === texto)

describe('ListaDeSolicitudes', () => {
  it('lista cada solicitud con quien la pidio', async () => {
    const w = montar()
    await flushPromises()
    expect(w.text()).toContain('Ana')
    expect(w.text()).toContain('ana@casa.com')
    expect(w.text()).toContain('2026-09-18 10:00')
  })

  it('asignar abre con una generada, la manda para su cuenta y la quita de la lista', async () => {
    const asignar = vi.fn(async () => {})
    const w = montar({ asignar })
    await flushPromises()

    await w.find('[data-solicitud="s1"]').trigger('click')
    await flushPromises()
    const campo = document.querySelector('[data-campo="password"] input') as HTMLInputElement
    const generada = campo.value
    expect(generada).toHaveLength(16)
    // A la vista: hay que poder leerla antes de pasarsela a nadie.
    expect(campo.type).toBe('text')

    botonEnDocumento('Asignar')!.click()
    await flushPromises()

    expect(asignar).toHaveBeenCalledWith('u1', generada)
    expect(w.find('[data-solicitud="s1"]').exists()).toBe(false)
    // Sigue a la vista despues de asignarla: es el unico momento para copiarla.
    expect(document.body.textContent).toContain('No se vuelve a mostrar')
  })

  // La regla de poder del api: el mensaje dice a quien pedirselo.
  it('un 403 de "tiene mas poder" se muestra con su mensaje y no quita la fila', async () => {
    const asignar = vi.fn(async () => {
      throw fallo(403, { message: 'Esa cuenta tiene permisos que tu no tienes. Pidele a un superadmin que le asigne la contrasena' })
    })
    const w = montar({ asignar })
    await flushPromises()
    await w.find('[data-solicitud="s1"]').trigger('click')
    await flushPromises()
    botonEnDocumento('Asignar')!.click()
    await flushPromises()

    expect(document.querySelector('[data-aviso="error"]')?.textContent).toContain('Pidele a un superadmin')
    expect(w.find('[data-solicitud="s1"]').exists()).toBe(true)
  })

  it('vacia dice de donde salen y deja volver a cargar', async () => {
    const cargar = vi.fn(async () => pagina([]))
    const w = montar({ cargar })
    await flushPromises()
    expect(w.text()).toContain('No hay solicitudes pendientes')
    await w.findAll('button').find((b) => b.text().includes('Volver a cargar'))!.trigger('click')
    expect(cargar).toHaveBeenCalledTimes(2)
  })

  it('un 403 al cargar dice que permiso falta y no ofrece reintentar', async () => {
    const w = montar({ cargar: async () => { throw fallo(403) } })
    await flushPromises()
    expect(w.text()).toContain('identity.user.password')
    expect(w.findAll('button').some((b) => b.text().includes('Reintentar'))).toBe(false)
  })

  it('carga con esqueleto', () => {
    const w = montar({ cargar: () => new Promise<never>(() => {}) })
    expect(w.find('[data-estado="cargando"]').exists()).toBe(true)
  })
})
