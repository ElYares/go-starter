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

const leer = { key: 'content.page.read', description: 'Ver las paginas', area: 'Paginas', sensitive: false }
const asignar = { key: 'identity.role.assign', description: 'Asignar roles', area: 'Usuarios y roles', sensitive: true }

const roles = pagina([
  { key: 'admin', name: 'Administracion', permissions: [leer] },
  { key: 'superadmin', name: 'Superadministracion', permissions: [leer, asignar] },
  { key: 'viewer', name: 'Solo lectura', permissions: [] },
])

async function montar(datos = roles) {
  const w = mount(ListaDeRoles, { props: { cargar: async () => datos } })
  await flushPromises()
  return w
}

describe('ListaDeRoles', () => {
  it('agrupa los permisos por area y pone la frase antes que la clave', async () => {
    const w = await montar()
    const super_ = w.find('[data-rol="superadmin"]')

    expect(super_.findAll('h3').map((h) => h.text())).toEqual(['Paginas', 'Usuarios y roles'])
    const fila = super_.find('[data-area="Paginas"] [data-permiso="content.page.read"]')
    expect(fila.text().indexOf('Ver las paginas')).toBeLessThan(fila.text().indexOf('content.page.read'))
  })

  it('resume cada rol: todo, una parte con lo que falta, o nada', async () => {
    const w = await montar()

    expect(w.find('[data-rol="superadmin"] [data-alcance]').text()).toBe('Puede todo lo que hay en el dashboard.')
    expect(w.find('[data-rol="admin"] [data-alcance]').text()).toContain('Le faltan 1 de 2 permisos, de Usuarios y roles')
    expect(w.find('[data-rol="viewer"] [data-alcance]').text()).toContain('no ve ninguna seccion')
    expect(w.find('[data-rol="viewer"] [data-area]').exists()).toBe(false)
  })

  it('a un rol parcial le muestra lo que no tiene, marcado y dicho para el lector', async () => {
    const w = await montar()
    const falta = w.find('[data-rol="admin"] [data-permiso="identity.role.assign"]')

    expect(falta.attributes('data-concedido')).toBe('false')
    expect(falta.classes()).toContain('falta')
    expect(falta.find('.solo-lector').text()).toBe('No puede:')
    expect(w.find('[data-rol="admin"] [data-permiso="content.page.read"] .solo-lector').text()).toBe('Puede:')
  })

  it('marca solo lo sensible y explica la marca una vez, arriba', async () => {
    const w = await montar()
    const super_ = w.find('[data-rol="superadmin"]')

    expect(super_.find('[data-permiso="content.page.read"]').text()).not.toContain('Da acceso')
    expect(super_.find('[data-permiso="identity.role.assign"]').text()).toContain('Da acceso')
    expect(w.find('[data-leyenda]').text()).toContain('dar o quitar acceso a otras personas')
  })

  it('sin nada sensible no hay leyenda que explicar', async () => {
    const w = await montar(pagina([{ key: 'admin', name: 'Administracion', permissions: [leer] }]))
    expect(w.find('[data-leyenda]').exists()).toBe(false)
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
