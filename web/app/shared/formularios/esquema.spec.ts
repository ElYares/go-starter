import { describe, expect, it } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import { erroresPorCampo, mover, resolverReferencias, type Esquema } from './esquema'

describe('mover', () => {
  it('intercambia con el vecino y no muta la lista original', () => {
    const lista = ['a', 'b', 'c']
    expect(mover(lista, 1, -1)).toEqual(['b', 'a', 'c'])
    expect(mover(lista, 1, 1)).toEqual(['a', 'c', 'b'])
    expect(lista).toEqual(['a', 'b', 'c'])
  })

  it('en los bordes no hace nada', () => {
    expect(mover(['a', 'b'], 0, -1)).toEqual(['a', 'b'])
    expect(mover(['a', 'b'], 1, 1)).toEqual(['a', 'b'])
  })
})

describe('los errores de un 400', () => {
  const fallo = new ApiError({
    status: 400,
    code: 'VALIDATION_FAILED',
    message: 'x',
    answered: true,
    unavailable: false,
    errors: [
      { field: 'blocks[1].props.title', code: 'required', message: 'Este campo es obligatorio' },
      { field: 'blocks[1].props.title', code: 'max', message: 'otro mensaje' },
      { field: 'blocks[10].type', code: 'unknown', message: 'No existe' },
      { field: 'slug', code: 'format', message: 'Solo minusculas' },
    ],
  })

  it('quedan por campo, con el primer mensaje de cada uno', () => {
    expect(erroresPorCampo(fallo)).toEqual({
      'blocks[1].props.title': 'Este campo es obligatorio',
      'blocks[10].type': 'No existe',
      slug: 'Solo minusculas',
    })
    expect(erroresPorCampo(null)).toEqual({})
  })

})

describe('resolverReferencias', () => {
  const enlace: Esquema = { type: 'object', title: 'Enlace', properties: { href: { type: 'string', title: 'Enlace' } } }

  it('sustituye un $ref a $defs, tambien dentro de una lista', () => {
    const r = resolverReferencias({
      type: 'object',
      properties: { links: { type: 'array', title: 'Enlaces', items: { $ref: '#/$defs/enlace' } } },
      $defs: { enlace },
    })
    expect(r.properties!.links!.items).toEqual(enlace)
  })

  // Lo que el sitio del $ref dice de mas —un title propio— gana sobre la
  // definicion: es lo que pide JSON Schema 2020-12.
  it('conserva lo que el sitio del $ref agrega', () => {
    const r = resolverReferencias({ type: 'array', items: { $ref: '#/$defs/enlace', title: 'Enlace del menu' }, $defs: { enlace } })
    expect(r.items!.title).toBe('Enlace del menu')
    expect(r.items!.properties).toEqual(enlace.properties)
  })

  it('una referencia que no encuentra, o que se nombra a si misma, se deja como esta', () => {
    expect(resolverReferencias({ type: 'array', items: { $ref: '#/$defs/nada' } }).items).toEqual({ $ref: '#/$defs/nada' })
    const ciclo = resolverReferencias({
      type: 'array',
      items: { $ref: '#/$defs/nodo' },
      $defs: { nodo: { type: 'array', items: { $ref: '#/$defs/nodo' } } },
    })
    expect(ciclo.items!.items).toEqual({ $ref: '#/$defs/nodo' })
  })
})
