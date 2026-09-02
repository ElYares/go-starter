import { describe, expect, it } from 'vitest'
import { resolveApiBase } from '../app/shared/api/base'

// La regla que esta prueba protege: en SSR la peticion NO puede salir hacia el
// dominio publico. Ese error da un ECONNREFUSED que solo aparece al recargar,
// nunca al navegar, y por eso se pierde media tarde buscandolo.
describe('resolveApiBase', () => {
  const input = { internal: 'http://api:8080/api/v1', publicBase: '/api/v1' }

  it('en el servidor usa la red interna del compose', () => {
    expect(resolveApiBase({ ...input, isServer: true })).toBe('http://api:8080/api/v1')
  })

  it('en el navegador usa el mismo origen', () => {
    expect(resolveApiBase({ ...input, isServer: false })).toBe('/api/v1')
  })
})
