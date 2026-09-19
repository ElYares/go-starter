// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import ListaDeRoles from './ListaDeRoles.vue'

const pagina = (content: Schemas['Rol'][]): Schemas['RolesPage'] => ({
  content,
  page: { number: 0, size: 100, totalElements: content.length, totalPages: 1 },
})

const roles = pagina([
  {
    key: 'superadmin',
    name: 'Superadministracion',
    permissions: [
      { key: 'content.page.read', description: 'Ver las paginas', sensitive: false },
      { key: 'identity.role.assign', description: 'Asignar roles', sensitive: true },
    ],
  },
  { key: 'viewer', name: 'Solo lectura', permissions: [] },
])

describe('ListaDeRoles', () => {
  it('muestra lo que concede cada rol y marca solo lo sensible', async () => {
    const w = mount(ListaDeRoles, { props: { cargar: async () => roles } })
    await flushPromises()

    const super_ = w.find('[data-rol="superadmin"]')
    expect(super_.text()).toContain('identity.role.assign')
    expect(super_.findAll('li')[0]!.text()).not.toContain('Reparte poder')
    expect(super_.findAll('li')[1]!.text()).toContain('Reparte poder')
    expect(w.find('[data-rol="viewer"]').text()).toContain('No concede ningun permiso')
  })

  it('sin roles dice que la migracion no corrio y deja volver a cargar', async () => {
    const cargar = vi.fn(async () => pagina([]))
    const w = mount(ListaDeRoles, { props: { cargar } })
    await flushPromises()

    expect(w.find('[data-estado="vacio"]').text()).toContain('migracion')
    await w.findAll('button').find((b) => b.text().includes('Volver a cargar'))!.trigger('click')
    expect(cargar).toHaveBeenCalledTimes(2)
  })

  it('un 403 dice que falta identity.role.read y no ofrece reintentar', async () => {
    const w = mount(ListaDeRoles, {
      props: {
        cargar: async () => {
          throw new ApiError({ status: 403, code: 'FORBIDDEN', message: 'x', answered: true, unavailable: false })
        },
      },
    })
    await flushPromises()
    expect(w.text()).toContain('identity.role.read')
    expect(w.findAll('button').some((b) => b.text().includes('Reintentar'))).toBe(false)
  })

  it('carga con esqueleto, no con la lista vacia', () => {
    const w = mount(ListaDeRoles, { props: { cargar: () => new Promise<never>(() => {}) } })
    expect(w.find('[data-estado="cargando"]').exists()).toBe(true)
    expect(w.find('[data-estado="vacio"]').exists()).toBe(false)
  })
})
