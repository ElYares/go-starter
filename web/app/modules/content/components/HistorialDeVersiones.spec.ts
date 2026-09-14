// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import HistorialDeVersiones from './HistorialDeVersiones.vue'

const pagina = (): Schemas['VersionesPage'] => ({
  content: [
    { id: 'v2', number: 2, title: 'T', note: null, published: true, createdAt: '2026-09-14T11:00:00Z', createdBy: null },
    { id: 'v1', number: 1, title: 'T', note: 'la primera', published: false, createdAt: '2026-09-13T11:00:00Z', createdBy: null },
  ],
  page: { number: 0, size: 100, totalElements: 2, totalPages: 1 },
})

describe('HistorialDeVersiones', () => {
  it('lista las versiones y solo ofrece publicar las no publicadas', async () => {
    const w = mount(HistorialDeVersiones, { props: { cargar: async () => pagina(), puedePublicar: true } })
    await flushPromises()

    expect(w.find('[data-version="2"]').text()).toContain('Publicada')
    expect(w.find('[data-version="2"]').find('button').exists()).toBe(false)
    expect(w.find('[data-version="1"]').text()).toContain('la primera')

    await w.find('[data-version="1"]').find('button').trigger('click')
    expect(w.emitted('publicar')).toEqual([['v1', 1]])
  })

  it('vuelve a pedir el historial cuando le avisan', async () => {
    let veces = 0
    const w = mount(HistorialDeVersiones, { props: { cargar: async () => (veces++, pagina()), puedePublicar: false } })
    await flushPromises()
    await w.setProps({ recarga: 1 })
    await flushPromises()
    expect(veces).toBe(2)
  })

  it('un fallo deja reintentar', async () => {
    let falla = true
    const w = mount(HistorialDeVersiones, {
      props: {
        cargar: async () => {
          if (falla) throw new ApiError({ status: 0, code: 'X', message: 'x', answered: false, unavailable: true })
          return pagina()
        },
        puedePublicar: true,
      },
    })
    await flushPromises()
    expect(w.text()).toContain('El servidor no responde')

    falla = false
    await w.findAll('button').find((b) => b.text() === 'Reintentar')!.trigger('click')
    await flushPromises()
    expect(w.findAll('[data-version]')).toHaveLength(2)
  })
})
