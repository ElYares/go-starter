import { describe, expect, it } from 'vitest'
import { ALFABETO, generarContrasena, LARGO_POR_OMISION } from './contrasena'

describe('generarContrasena', () => {
  it('tiene el largo pedido, y el minimo del api por omision', () => {
    expect(generarContrasena()).toHaveLength(LARGO_POR_OMISION)
    expect(LARGO_POR_OMISION).toBeGreaterThanOrEqual(12)
    expect(generarContrasena(40)).toHaveLength(40)
  })

  it('solo usa el alfabeto, que no tiene caracteres que se confunden', () => {
    for (const c of generarContrasena(500)) expect(ALFABETO).toContain(c)
    for (const confuso of '0O1lI') expect(ALFABETO).not.toContain(confuso)
  })

  it('dos seguidas no se repiten', () => {
    expect(generarContrasena()).not.toBe(generarContrasena())
  })

  // Sin descartar, `byte % n` favorece los primeros caracteres. Una fuente que
  // solo da bytes del tramo sobrante tiene que descartarse entera.
  it('descarta los bytes que meterian sesgo', () => {
    const tope = 256 - (256 % ALFABETO.length)
    let vuelta = 0
    const fuente = (bytes: Uint8Array) => {
      // Primero una tanda de puros bytes a descartar, despues bytes validos.
      bytes.fill(vuelta++ === 0 ? tope : 0)
      return bytes
    }
    expect(generarContrasena(12, fuente)).toBe(ALFABETO[0]!.repeat(12))
    expect(vuelta).toBe(2)
  })
})
