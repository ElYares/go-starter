import { readFileSync, readdirSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// El criterio de HU-007 que mas se erosiona solo: "no hay un solo color, radio o
// espacio literal". Se rompe de a un `#fff` por vez, cada uno con su prisa, y el
// dia que un fork cambia --color-accent la mitad de la interfaz no se entera.
//
// Que se comprueba y que no:
//
// - COLORES: ninguno literal, en ninguna propiedad. Las palabras que no son un
//   color (transparent, currentColor, inherit) siguen valiendo.
// - ESPACIO y RADIO: solo en las propiedades donde son decision de diseno
//   —padding, margin, gap, border-radius, inset—. El `1px` de un borde NO entra:
//   el grosor de una linea no es un token en ningun sistema serio, y meterlo
//   obligaria a inventar --border-1 sin que nadie lo cambie jamas.
// - Se permiten `0`, `auto` y los porcentajes: un `top: 50%` es una relacion, no
//   una medida del sistema de diseno.

const raizApp = fileURLToPath(new URL('../app', import.meta.url))

const COLOR_LITERAL =
  /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\s*\(|\b(?:white|black|red|green|blue|yellow|orange|purple|pink|brown|gray|grey|silver|gold|navy|teal|olive|lime|aqua|cyan|magenta|maroon|fuchsia|indigo|violet|beige|ivory|khaki|coral|salmon|crimson|tomato|tan|plum|azure|snow|wheat|linen)\b/

// Solo el lado del VALOR de cada declaracion. Mirar la linea entera hace saltar
// `white-space: nowrap` como si fuera el color blanco, y un guardia con falsos
// positivos se desactiva a la semana.
const DECLARACION = /^\s*([a-z-]+)\s*:\s*([^;]+);/

const PROPIEDADES_DE_ESPACIO = /^(padding|margin|gap|row-gap|column-gap|border-radius|inset)(-[a-z-]+)?$/

const MEDIDA_LITERAL = /(?<![\w-])\d*\.?\d+(px|rem|em|ch|ex|vw|vh|vmin|vmax)\b/

function vistas(dir: string): string[] {
  const out: string[] = []
  for (const entrada of readdirSync(dir, { withFileTypes: true })) {
    const ruta = join(dir, entrada.name)
    if (entrada.isDirectory()) out.push(...vistas(ruta))
    else if (entrada.name.endsWith('.vue')) out.push(ruta)
  }
  return out
}

/** Solo el contenido de los bloques <style>: el marcado y el script no cuentan. */
function estilos(fuente: string): string {
  return [...fuente.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]).join('\n')
}

/** Fuera comentarios: un color mencionado al explicar algo no es un color usado. */
function sinComentarios(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

const archivos = vistas(raizApp).map((ruta) => ({
  ruta: relative(raizApp, ruta),
  css: sinComentarios(estilos(readFileSync(ruta, 'utf8'))),
}))

describe('los componentes no llevan valores crudos', () => {
  it('hay CSS que revisar', () => {
    // Sin esto, un cambio en como se leen los <style> dejaria la prueba pasando
    // sobre cadenas vacias, que es la forma mas comoda de tener un guardia que
    // no guarda nada.
    expect(archivos.filter((a) => a.css.trim().length > 0).length).toBeGreaterThan(5)
  })

  for (const { ruta, css } of archivos) {
    describe(ruta, () => {
      it('no tiene colores literales', () => {
        const lineas = css
          .split('\n')
          .map((l, i) => [i + 1, l] as const)
          .filter(([, l]) => {
            const m = DECLARACION.exec(l)
            return m ? COLOR_LITERAL.test(m[2]!) : false
          })
          .map(([n, l]) => `  ${n}: ${l.trim()}`)

        expect(
          lineas,
          `colores escritos a mano en ${ruta}:\n${lineas.join('\n')}\n` +
            'usa un token de assets/tokens/base.css; si el color que falta no existe, ' +
            'agregalo alli con su par de texto y su valor de tema oscuro.',
        ).toEqual([])
      })

      it('no tiene espacios ni radios literales', () => {
        const lineas = css
          .split('\n')
          .map((l, i) => [i + 1, l] as const)
          .filter(([, l]) => {
            const m = DECLARACION.exec(l)
            return m ? PROPIEDADES_DE_ESPACIO.test(m[1]!) && MEDIDA_LITERAL.test(m[2]!) : false
          })
          .map(([n, l]) => `  ${n}: ${l.trim()}`)

        expect(
          lineas,
          `medidas escritas a mano en ${ruta}:\n${lineas.join('\n')}\n` +
            'usa --space-* o --radius-*. Un valor que no encaja en la escala es la ' +
            'senal de que falta un peldano en la escala, no de que este caso es especial.',
        ).toEqual([])
      })
    })
  }
})
