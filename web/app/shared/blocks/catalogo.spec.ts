import { readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { esquemas } from './catalogo'
import { bloques } from './registry'

const carpeta = fileURLToPath(new URL('../../../../api/internal/modules/content/bloques', import.meta.url))

describe('el catalogo importado', () => {
  // Si el glob apuntara mal, `esquemas` quedaria vacio y el editor no ofreceria
  // ningun bloque, sin un solo error.
  it('trae un esquema por archivo del api, con el tipo como nombre', () => {
    const archivos = readdirSync(carpeta).filter((f) => f.endsWith('.json')).map((f) => f.slice(0, -5)).sort()
    expect(archivos.length).toBeGreaterThan(0)
    expect(Object.keys(esquemas).sort()).toEqual(archivos)
  })

  it('cada tipo con esquema tiene componente, y al reves', () => {
    expect(Object.keys(esquemas).sort()).toEqual(Object.keys(bloques).sort())
  })

  // El formulario generado usa `title` como etiqueta. Sin el, el editor
  // pintaria `subtitle` o `href`, que no le dicen nada a quien edita.
  it('toda propiedad del catalogo trae su etiqueta', () => {
    const sinEtiqueta: string[] = []
    const revisar = (esquema: (typeof esquemas)[string], ruta: string) => {
      for (const [nombre, prop] of Object.entries(esquema.properties ?? {})) {
        if (!prop.title) sinEtiqueta.push(`${ruta}.${nombre}`)
        revisar(prop, `${ruta}.${nombre}`)
        if (prop.items) {
          // El elemento de una lista tambien: es el "Punto 1" de sus botones.
          if (!prop.items.title) sinEtiqueta.push(`${ruta}.${nombre}[]`)
          revisar(prop.items, `${ruta}.${nombre}[]`)
        }
      }
    }
    for (const [tipo, esquema] of Object.entries(esquemas)) revisar(esquema, tipo)
    expect(sinEtiqueta).toEqual([])
  })
})
