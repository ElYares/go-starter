// @vitest-environment happy-dom
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import ListaDePaginas from './ListaDePaginas.vue'

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

const resumen = (slug: string, extra: Partial<Schemas['PaginaResumen']> = {}): Schemas['PaginaResumen'] => ({
  id: `id-${slug}`,
  slug,
  title: slug.toUpperCase(),
  version: 1,
  draftNumber: 2,
  publishedNumber: 2,
  updatedAt: '2026-09-14T10:00:00Z',
  updatedBy: null,
  ...extra,
})

const pagina = (content: Schemas['PaginaResumen'][]): Schemas['PaginasPage'] => ({
  content,
  page: { number: 0, size: 100, totalElements: content.length, totalPages: 1 },
})

function montar(props: Partial<InstanceType<typeof ListaDePaginas>['$props']> = {}) {
  const w = mount(ListaDePaginas, {
    props: {
      cargar: async () => pagina([resumen('inicio'), resumen('precios', { publishedNumber: 1 }), resumen('nueva', { publishedNumber: null })]),
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

describe('ListaDePaginas', () => {
  it('lista cada pagina con su enlace al editor, su direccion y su estado', async () => {
    const w = montar()
    await flushPromises()

    expect(w.findAllComponents(RouterLinkStub).map((l) => l.props('to'))).toEqual([
      '/admin/paginas/id-inicio',
      '/admin/paginas/id-precios',
      '/admin/paginas/id-nueva',
    ])
    expect(w.text()).toContain('/precios')
    // La portada vive en /, no en /inicio.
    expect(w.findAll('code').map((c) => c.text())).toContain('/')
    expect(w.text()).toContain('Publicada')
    expect(w.text()).toContain('Cambios sin publicar')
    expect(w.text()).toContain('Sin publicar')
  })

  it('vacio invita a crear la primera, si se puede', async () => {
    const w = montar({ cargar: async () => pagina([]) })
    await flushPromises()
    expect(w.text()).toContain('Todavia no hay paginas')
    expect(w.findAll('button').some((b) => b.text().includes('Crear la primera pagina'))).toBe(true)
  })

  it('un 403 dice que falta content.page.read y no ofrece reintentar', async () => {
    const w = montar({
      cargar: async () => {
        throw new ApiError({ status: 403, code: 'FORBIDDEN', message: 'x', answered: true, unavailable: false })
      },
    })
    await flushPromises()
    expect(w.text()).toContain('No tienes permiso para ver las paginas')
    expect(w.findAll('button').some((b) => b.text().includes('Reintentar'))).toBe(false)
  })

  it('sin content.page.write no ofrece crear', async () => {
    const w = montar({ puedeEscribir: false })
    await flushPromises()
    expect(w.findAll('button').some((b) => b.text().includes('Nueva pagina'))).toBe(false)
  })

  it('el alta sugiere el slug desde el titulo, crea sin bloques y avisa', async () => {
    const creada = { id: 'nuevo' } as Schemas['Pagina']
    const crear = vi.fn(async () => creada)
    const w = montar({ crear })
    await flushPromises()

    await w.findAll('button').find((b) => b.text().includes('Nueva pagina'))!.trigger('click')
    await flushPromises()

    const titulo = enDocumento('[data-campo="title"]')!
    titulo.value = 'Quiénes somos'
    titulo.dispatchEvent(new Event('input'))
    await flushPromises()
    expect(enDocumento('[data-campo="slug"]')!.value).toBe('quienes-somos')

    botonEnDocumento('Crear pagina')!.click()
    await flushPromises()

    expect(crear).toHaveBeenCalledWith({ title: 'Quiénes somos', slug: 'quienes-somos', blocks: [] })
    expect(w.emitted('creada')).toEqual([[creada]])
  })

  it('un slug ocupado se marca en su campo y el dialogo sigue abierto', async () => {
    const crear = vi.fn(async () => {
      throw new ApiError({ status: 409, code: 'CONFLICT', message: 'x', answered: true, unavailable: false })
    })
    const w = montar({ crear })
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('Nueva pagina'))!.trigger('click')
    await flushPromises()

    botonEnDocumento('Crear pagina')!.click()
    await flushPromises()

    expect(document.body.textContent).toContain('Ya hay una pagina con esta direccion')
    expect(enDocumento('[data-campo="slug"]')!.getAttribute('aria-invalid')).toBe('true')
    expect(w.emitted('creada')).toBeUndefined()
  })
})
