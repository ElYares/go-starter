import { describe, expect, it } from 'vitest'
import { sitioDe } from './sitio'

const mapa = {
  'site.brand': { name: 'go-starter', tagline: 'Una landing', logo: '01a0b5e1-0e44-78ab-8df9-a128bc8c9346' },
  'site.nav': [
    { label: 'Inicio', href: '/' },
    { label: 'Precios', href: '/precios' },
    { label: 'Blog', href: 'https://blog.ejemplo.com' },
  ],
  'site.footer': { text: 'Hecho con go-starter', links: [{ label: 'Aviso', href: '/aviso' }] },
}

describe('sitioDe', () => {
  it('arma marca, menu y pie desde el mapa publico', () => {
    expect(sitioDe(mapa)).toEqual({
      nombre: 'go-starter',
      lema: 'Una landing',
      logo: '01a0b5e1-0e44-78ab-8df9-a128bc8c9346',
      nav: mapa['site.nav'],
      pie: { texto: 'Hecho con go-starter', enlaces: [{ label: 'Aviso', href: '/aviso' }] },
    })
  })

  it('sin nombre no hay marca que pintar', () => {
    expect(sitioDe({ 'site.nav': mapa['site.nav'] })).toBeNull()
    expect(sitioDe({ 'site.brand': { name: '  ' } })).toBeNull()
    expect(sitioDe(null)).toBeNull()
  })

  it('sin menu ni pie, quedan vacios y no rompen', () => {
    expect(sitioDe({ 'site.brand': { name: 'X' } })).toEqual({
      nombre: 'X',
      lema: undefined,
      logo: undefined,
      nav: [],
      pie: { texto: undefined, enlaces: [] },
    })
  })

  // El api valida contra el esquema, pero una fila escrita a mano no pasa por
  // el: lo que sale de aqui es un href en todas las paginas del sitio.
  it('descarta a la defensiva los enlaces que el esquema no dejaria pasar', () => {
    const s = sitioDe({
      'site.brand': { name: 'X' },
      'site.nav': [
        { label: 'Malo', href: 'javascript:alert(1)' },
        { label: 'Afuera', href: '//otro.com' },
        { label: 'Plano', href: 'http://otro.com' },
        { label: '', href: '/vacio' },
        { href: '/sin-texto' },
        'no soy un enlace',
        { label: 'Bueno', href: '/bueno' },
      ],
    })
    expect(s!.nav).toEqual([{ label: 'Bueno', href: '/bueno' }])
  })
})
