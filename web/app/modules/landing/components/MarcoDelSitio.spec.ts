// @vitest-environment happy-dom
import { mount, RouterLinkStub } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Sitio } from '../sitio'
import CabeceraDelSitio from './CabeceraDelSitio.vue'
import PieDelSitio from './PieDelSitio.vue'

// El marco que pide la configuracion (MarcoDelSitio) necesita a Nuxt y se
// comprueba contra el stack: con curl, y con la configuracion rota a proposito.
// Aqui se prueba lo que pinta cada pieza con un sitio dado.

const sitio = (extra: Partial<Sitio> = {}): Sitio => ({
  nombre: 'go-starter',
  nav: [
    { label: 'Inicio', href: '/' },
    { label: 'Precios', href: '/precios' },
    { label: 'Blog', href: 'https://blog.ejemplo.com' },
  ],
  pie: { texto: 'Hecho con go-starter', enlaces: [{ label: 'Aviso', href: '/aviso' }] },
  ...extra,
})

const global = { stubs: { NuxtLink: RouterLinkStub } }
const urlDeLogo = (id: string) => `/api/v1/public/media/${id}`

describe('CabeceraDelSitio', () => {
  it('con logo, pinta la imagen con el nombre en alt', () => {
    const w = mount(CabeceraDelSitio, { props: { sitio: sitio({ logo: 'abc' }), urlDeLogo }, global })
    const img = w.find('img')
    expect(img.attributes('src')).toBe('/api/v1/public/media/abc')
    expect(img.attributes('alt')).toBe('go-starter')
  })

  it('sin logo, el nombre como texto y ninguna imagen rota', () => {
    const w = mount(CabeceraDelSitio, { props: { sitio: sitio(), urlDeLogo }, global })
    expect(w.find('img').exists()).toBe(false)
    expect(w.find('.nombre').text()).toBe('go-starter')
  })

  it('el menu sale dentro de un nav, en orden', () => {
    const w = mount(CabeceraDelSitio, { props: { sitio: sitio(), urlDeLogo }, global })
    // Los del menu: el primero de la cabecera es el de la marca, fuera del nav.
    const enlaces = w.findAllComponents(RouterLinkStub).slice(1)
    expect(enlaces.map((l) => [l.text(), l.props('to')])).toEqual([
      ['Inicio', '/'],
      ['Precios', '/precios'],
      ['Blog', 'https://blog.ejemplo.com'],
    ])
    expect(w.find('nav').attributes('aria-label')).toBe('Principal')
  })

  it('sin menu no pinta un nav vacio', () => {
    const w = mount(CabeceraDelSitio, { props: { sitio: sitio({ nav: [] }), urlDeLogo }, global })
    expect(w.find('nav').exists()).toBe(false)
  })
})

describe('PieDelSitio', () => {
  it('pinta el texto y los enlaces dentro de un footer', () => {
    const w = mount(PieDelSitio, { props: { sitio: sitio() }, global })
    expect(w.find('footer').text()).toContain('Hecho con go-starter')
    expect(w.findAllComponents(RouterLinkStub).map((l) => l.props('to'))).toEqual(['/aviso'])
  })

  it('sin texto ni enlaces no pinta un footer vacio', () => {
    const w = mount(PieDelSitio, { props: { sitio: sitio({ pie: { enlaces: [] } }) }, global })
    expect(w.find('footer').exists()).toBe(false)
  })
})
