// @vitest-environment happy-dom
import { mount, RouterLinkStub } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Perfil } from '~/modules/auth/sesion'
import AdminShell from './AdminShell.vue'

function perfil(permissions: string[]): Perfil {
  return { id: 'u', email: 'a@b.c', displayName: 'Ana', roles: [], permissions }
}

function montar(permissions: string[]) {
  return mount(AdminShell, {
    props: { perfil: perfil(permissions) },
    slots: { default: '<p class="dentro">contenido</p>' },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

const rutas = (w: ReturnType<typeof montar>) => w.findAllComponents(RouterLinkStub).map((l) => l.props('to'))

describe('AdminShell', () => {
  // CU-003 criterio 6, la mitad del cliente: la entrada no aparece.
  it('sin settings.read no muestra la entrada de configuracion', () => {
    expect(rutas(montar([]))).toEqual(['/admin'])
  })

  it('con settings.read la muestra', () => {
    expect(rutas(montar(['settings.read']))).toEqual(['/admin', '/admin/configuracion'])
  })

  it('dice quien tiene la sesion y pinta el contenido', () => {
    const w = montar([])
    expect(w.text()).toContain('Sesion de Ana')
    expect(w.find('.dentro').exists()).toBe(true)
  })

  it('avisa al pulsar cerrar sesion', async () => {
    const w = montar([])
    await w.findAll('button').find((b) => b.text().includes('Cerrar sesion'))!.trigger('click')
    expect(w.emitted('salir')).toHaveLength(1)
  })

  it('la navegacion tiene nombre para el lector de pantalla', () => {
    expect(montar([]).find('nav').attributes('aria-label')).toBeTruthy()
  })
})
