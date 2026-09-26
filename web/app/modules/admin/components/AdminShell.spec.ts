// @vitest-environment happy-dom
import { mount, RouterLinkStub, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import type { Perfil } from '~/modules/auth/sesion'
import AdminShell from './AdminShell.vue'

function perfil(permissions: string[]): Perfil {
  return { id: 'u', email: 'a@b.c', displayName: 'Ana', roles: [], permissions, mustChangePassword: false }
}

function montar(permissions: string[], opciones: { attachTo?: HTMLElement } = {}) {
  return mount(AdminShell, {
    props: { perfil: perfil(permissions) },
    slots: { default: '<p class="dentro">contenido</p>' },
    global: { stubs: { RouterLink: RouterLinkStub } },
    ...opciones,
  })
}

const boton = (w: ReturnType<typeof montar>, nombre: string) => w.find(`button[aria-label="${nombre}"]`)

// Solo las secciones: la barra de arriba tiene su propio enlace.
const rutas = (w: ReturnType<typeof montar>) => w.find('nav').findAllComponents(RouterLinkStub).map((l: VueWrapper) => (l.props() as { to: unknown }).to)

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

  // HU-019: cambiar la propia no pide permiso, asi que esta siempre.
  it('ofrece cambiar la propia contrasena, sin importar los permisos', () => {
    const enlaces = montar([]).find('header').findAllComponents(RouterLinkStub).map((l: VueWrapper) => (l.props() as { to: unknown }).to)
    expect(enlaces).toContain('/admin/contrasena')
  })

  it('la navegacion tiene nombre para el lector de pantalla', () => {
    expect(montar([]).find('nav').attributes('aria-label')).toBeTruthy()
  })

  describe('menu lateral en ancho', () => {
    beforeEach(() => localStorage.clear())

    it('arranca visible y el boton lo oculta y lo vuelve a mostrar', async () => {
      const w = montar([])
      expect(w.find('.shell').classes()).not.toContain('plegado')
      expect(boton(w, 'Ocultar menu').attributes('aria-expanded')).toBe('true')

      await boton(w, 'Ocultar menu').trigger('click')
      expect(w.find('.shell').classes()).toContain('plegado')
      expect(boton(w, 'Mostrar menu').attributes('aria-expanded')).toBe('false')

      await boton(w, 'Mostrar menu').trigger('click')
      expect(w.find('.shell').classes()).not.toContain('plegado')
    })

    it('el boton dice que menu controla', () => {
      const w = montar([])
      expect(boton(w, 'Ocultar menu').attributes('aria-controls')).toBe(w.find('aside').attributes('id'))
    })

    it('se acuerda de como lo dejaste', async () => {
      await boton(montar([]), 'Ocultar menu').trigger('click')
      await nextTick()
      expect(montar([]).find('.shell').classes()).toContain('plegado')
    })

    it('sin almacenamiento arranca visible y se sigue pudiendo ocultar', async () => {
      const lanza = () => {
        throw new DOMException('bloqueado', 'SecurityError')
      }
      // El global entero y no un spy de Storage.prototype: el localStorage de
      // happy-dom no pasa por el prototipo, y el spy no lanzaba nunca.
      vi.stubGlobal('localStorage', { getItem: lanza, setItem: lanza, removeItem: lanza })
      try {
        const w = montar([])
        expect(w.find('.shell').classes()).not.toContain('plegado')
        await boton(w, 'Ocultar menu').trigger('click')
        expect(w.find('.shell').classes()).toContain('plegado')
      } finally {
        vi.unstubAllGlobals()
      }
    })
  })

  // El cajon se monta en un portal, fuera del wrapper: se busca en el body.
  describe('cajon en angosto', () => {
    const cajon = () => document.body.querySelector<HTMLElement>('[role="dialog"]')
    let w: ReturnType<typeof montar>

    beforeEach(async () => {
      w = montar(['settings.read'], { attachTo: document.body })
      await boton(w, 'Abrir menu').trigger('click')
      await nextTick()
    })
    afterEach(() => w.unmount())

    it('la hamburguesa abre las secciones y la sesion', () => {
      const abierto = cajon()
      expect(abierto).not.toBeNull()
      const rutas = [...abierto!.querySelectorAll('nav a')].map((a) => a.textContent?.trim())
      expect(rutas).toEqual(['Inicio', 'Configuracion'])
      expect(abierto!.textContent).toContain('Sesion de Ana')
      expect(abierto!.textContent).toContain('Mi contrasena')
    })

    it('ir a una seccion cierra el cajon', async () => {
      cajon()!.querySelector<HTMLElement>('nav a')!.click()
      await nextTick()
      await nextTick()
      // Reka deja el nodo hasta que termine la animacion de salida, que en
      // happy-dom no termina nunca: lo que cuenta es que paso a cerrado.
      expect(cajon()?.getAttribute('data-state') ?? 'closed').toBe('closed')
    })

    it('cerrar sesion desde el cajon avisa', async () => {
      const salir = [...cajon()!.querySelectorAll('button')].find((b) => b.textContent?.includes('Cerrar sesion'))
      salir!.click()
      await nextTick()
      expect(w.emitted('salir')).toHaveLength(1)
    })
  })
})
