import { describe, expect, it } from 'vitest'
import { esquemas } from '~/modules/settings/esquemas'
import { esquemas as bloques } from './catalogo'
import { esHrefSeguro, HREF_SEGURO } from './href'

describe('esHrefSeguro', () => {
  it('rechaza lo que lleva a otro sitio o ejecuta algo', () => {
    for (const href of [
      '//otro.com',
      '/\\otro.com',
      '/\t/otro.com',
      '/\n/otro.com',
      'http://otro.com',
      'javascript:alert(1)',
      'data:text/html,x',
      'precios',
      '',
      undefined,
      7,
    ]) {
      expect(esHrefSeguro(href), JSON.stringify(href)).toBe(false)
    }
  })

  it('acepta rutas del sitio y https', () => {
    for (const href of ['/', '/admin', '/precios#planes', '/a//b', 'https://otro.com']) {
      expect(esHrefSeguro(href), href).toBe(true)
    }
  })

  // La defensa del navegador y la validacion del api tienen que ser la misma
  // regla. Si alguien afloja un esquema y no esto —o al reves—, la landing
  // esconde enlaces que el api acepto, o pinta los que el api rechazaria.
  it('es el mismo patron que los esquemas del api', () => {
    const patrones = [
      bloques.hero?.properties?.cta?.properties?.href?.pattern,
      esquemas['site.nav']?.items?.properties?.href?.pattern,
      esquemas['site.footer']?.properties?.links?.items?.properties?.href?.pattern,
    ]
    expect(patrones).toEqual(Array(3).fill(HREF_SEGURO.source.replaceAll('\\/', '/')))
  })
})
