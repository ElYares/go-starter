import { afterEach, describe, expect, it, vi } from 'vitest'
import { useApiFetch } from './useApiFetch'

// useApiFetch se apoya en los auto-imports de Nuxt, que en estas pruebas no
// existen: se ponen como globales, y `useFetch` devuelve las opciones que
// recibio.
function llamar(apiBase: string, apiInternal: string, path = '/public/pages/inicio') {
  vi.stubGlobal('useRuntimeConfig', () => ({ apiInternal, public: { apiBase } }))
  vi.stubGlobal('useFetch', (_path: string, opciones: Record<string, unknown>) => opciones)
  return useApiFetch(path) as unknown as Record<string, unknown>
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useApiFetch', () => {
  // El SSR guarda la respuesta en el payload bajo este key y el navegador la
  // busca con el suyo. Si el key depende del baseURL —el de Nuxt lo hace—, no
  // coinciden nunca, porque el SSR sale por http://api:8080 y el navegador por
  // /api/v1: la landing hidrataba sin datos y pintaba un 503.
  it('el key no depende de la base de la API', () => {
    const a = llamar('/api/v1', 'http://api:8080/api/v1')
    const b = llamar('https://otro.example/api/v1', 'http://interno:9000/api/v1')

    expect(a.key).toBe('api:/public/pages/inicio')
    expect(b.key).toBe(a.key)
  })

  it('cada ruta tiene su key', () => {
    expect(llamar('/api/v1', 'x', '/public/settings').key).not.toBe(llamar('/api/v1', 'x').key)
  })
})
