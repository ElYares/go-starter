import { readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { bloques, resolverBloque } from './registry'

// El catalogo del api: un JSON Schema por tipo. En el contenedor de web lo
// monta compose.yaml en la misma ruta relativa; en CI es el checkout.
const catalogo = fileURLToPath(new URL('../../../../api/internal/modules/content/bloques', import.meta.url))

describe('el registro de bloques', () => {
  // Un tipo son dos archivos: su esquema en el api y su componente aqui. Si el
  // api acepta un tipo que la landing no sabe pintar, la pagina se publica y el
  // bloque desaparece en silencio. Al reves, el editor ofreceria un bloque que
  // el servidor rechaza al guardar.
  it('tiene exactamente los tipos del catalogo del api', () => {
    const delApi = readdirSync(catalogo)
      .filter((f) => f.endsWith('.json'))
      .map((f) => f.replace(/\.json$/, ''))
      .sort()

    // Un guardia que pasa por vacuidad no guarda nada: si no leyo el catalogo,
    // falla aqui y no en la comparacion.
    expect(delApi.length).toBeGreaterThan(0)
    expect(Object.keys(bloques).sort()).toEqual(delApi)
  })

  it('cada tipo tiene componente y un nombre para el editor', () => {
    for (const [tipo, def] of Object.entries(bloques)) {
      expect(def.component, tipo).toBeTruthy()
      expect(def.label.trim(), tipo).not.toBe('')
    }
  })

  it('un tipo que no esta resuelve a null, tambien si se llama como algo del prototipo', () => {
    expect(resolverBloque('hero')).toBe(bloques.hero)
    for (const tipo of ['carrusel', 'constructor', 'toString', '__proto__', '']) {
      expect(resolverBloque(tipo), tipo).toBeNull()
    }
  })
})
