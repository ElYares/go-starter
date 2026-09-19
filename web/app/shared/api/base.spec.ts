import { describe, expect, it } from 'vitest'
import { cabecerasDelSSR, ipDelVisitante } from './base'

describe('ipDelVisitante', () => {
  // La primera entrada la escribe el cliente; la ultima, el edge.
  it('toma la ultima entrada de X-Forwarded-For, no la primera', () => {
    expect(ipDelVisitante('1.2.3.4, 203.0.113.7', '172.18.0.2')).toBe('203.0.113.7')
    expect(ipDelVisitante(' 203.0.113.7 ', '172.18.0.2')).toBe('203.0.113.7')
  })

  it('sin la cabecera, la conexion', () => {
    expect(ipDelVisitante(undefined, '172.18.0.2')).toBe('172.18.0.2')
    expect(ipDelVisitante('', '172.18.0.2')).toBe('172.18.0.2')
    expect(ipDelVisitante(undefined, undefined)).toBeUndefined()
  })
})

describe('cabecerasDelSSR', () => {
  it('manda el secreto y la IP juntos', () => {
    expect(cabecerasDelSSR('s', '203.0.113.7')).toEqual({ 'X-SSR-Secret': 's', 'X-SSR-Client-IP': '203.0.113.7' })
  })

  it('sin alguno de los dos no manda nada', () => {
    expect(cabecerasDelSSR('', '203.0.113.7')).toEqual({})
    expect(cabecerasDelSSR(undefined, '203.0.113.7')).toEqual({})
    expect(cabecerasDelSSR('s', undefined)).toEqual({})
  })
})
