import { describe, expect, it } from 'vitest'
import { codigoDeFallo, metaDePagina, SLUG_DE_PORTADA, slugDeRuta, statusRespondido } from './pagina'

describe('slugDeRuta', () => {
  it('la raiz es la portada', () => {
    for (const segmentos of [undefined, '', [], ['']]) {
      expect(slugDeRuta(segmentos), JSON.stringify(segmentos)).toBe(SLUG_DE_PORTADA)
    }
  })

  it('un segmento valido es su slug', () => {
    expect(slugDeRuta(['precios'])).toBe('precios')
    expect(slugDeRuta('sobre-nosotros')).toBe('sobre-nosotros')
    // Un slash final deja un segmento vacio detras: sigue siendo la misma pagina.
    expect(slugDeRuta(['precios', ''])).toBe('precios')
  })

  // Lo que no puede existir en la base no se le pregunta a la API.
  it('lo que no cumple el patron del contrato no es una pagina', () => {
    for (const segmentos of [['Precios'], ['con espacio'], ['-precios'], ['a--b'], ['nosotros', 'equipo'], ['x'.repeat(81)]]) {
      expect(slugDeRuta(segmentos), JSON.stringify(segmentos)).toBeNull()
    }
  })
})

describe('codigoDeFallo', () => {
  it('sin respuesta es 503: la API esta caida', () => {
    expect(codigoDeFallo(undefined)).toBe(503)
    expect(codigoDeFallo(0)).toBe(503)
  })

  it('una puerta de enlace sin servicio detras tambien es 503', () => {
    for (const s of [502, 503, 504]) expect(codigoDeFallo(s)).toBe(503)
  })

  it('no existe o slug rechazado es 404, nunca 200', () => {
    expect(codigoDeFallo(404)).toBe(404)
    expect(codigoDeFallo(400)).toBe(404)
  })

  it('un fallo nuestro es 500, no "vuelve mas tarde"', () => {
    expect(codigoDeFallo(500)).toBe(500)
    expect(codigoDeFallo(401)).toBe(500)
  })
})

// Las dos formas que entrega useFetch, tal como se vieron en la stack local.
describe('statusRespondido', () => {
  it('con la API apagada no hay status, aunque el error diga 500', () => {
    const caida = { statusCode: 500, cause: new TypeError('fetch failed') }
    expect(statusRespondido(caida)).toBeUndefined()
    expect(codigoDeFallo(statusRespondido(caida))).toBe(503)
  })

  it('con respuesta, el status es el de la respuesta', () => {
    const noExiste = { statusCode: 404, cause: { name: 'FetchError', statusCode: 404, response: { status: 404 } } }
    expect(statusRespondido(noExiste)).toBe(404)
  })

  it('sin error ni causa no inventa nada', () => {
    expect(statusRespondido(null)).toBeUndefined()
    expect(statusRespondido({})).toBeUndefined()
  })
})

describe('metaDePagina', () => {
  const base = { slug: 'precios', title: 'Precios', seoTitle: null, seoDescription: null, blocks: [] }

  it('el titulo SEO manda sobre el titulo, en el head y en Open Graph', () => {
    const meta = metaDePagina({ ...base, seoTitle: 'Precios claros', seoDescription: 'Sin letra chica' }, 'http://x/precios')
    expect(meta).toEqual({
      title: 'Precios claros',
      description: 'Sin letra chica',
      ogTitle: 'Precios claros',
      ogDescription: 'Sin letra chica',
      ogType: 'website',
      ogUrl: 'http://x/precios',
    })
  })

  it('sin SEO usa el titulo y no inventa una descripcion', () => {
    const meta = metaDePagina(base)
    expect(meta.title).toBe('Precios')
    expect(meta.description).toBeUndefined()
    expect(meta.ogDescription).toBeUndefined()
  })
})
