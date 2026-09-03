import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Por que esto es una prueba y no un vistazo de una tarde:
//
// El par --color-accent / --color-on-accent en tema claro da 4.53:1. Tres
// centesimas por encima del minimo. Lo primero que hace un fork es meter su
// color de marca en ese token, y no hay ninguna senal de que lo rompio: la
// landing se ve bien, nadie se queja, y el texto sobre el boton principal deja
// de leerse para quien lo necesita. Con esta prueba, romperlo es rojo en el CI.
//
// Se leen los tokens del CSS de verdad y no de una copia en TypeScript, porque
// una copia se desincroniza el primer dia y entonces la prueba mide un archivo
// que ya no es la piel del proyecto.

const css = readFileSync(
  fileURLToPath(new URL('../app/assets/tokens/base.css', import.meta.url)),
  'utf8',
)

/** Los pares que tienen que leerse. Izquierda pinta sobre derecha. */
const PARES: Array<[string, string]> = [
  ['--color-text', '--color-bg'],
  ['--color-text', '--color-surface'],
  ['--color-text', '--color-surface-hover'],
  ['--color-text-muted', '--color-bg'],
  ['--color-text-muted', '--color-surface'],
  ['--color-text-muted', '--color-surface-hover'],
  ['--color-accent-strong', '--color-bg'],
  ['--color-accent-strong', '--color-surface'],
  ['--color-on-accent', '--color-accent'],
  // El acento fuerte tambien se usa como relleno (item seleccionado de un
  // select, boton en foco), y entonces necesita su par de texto igual que el
  // de marca. Es la regla "todo relleno tiene su par" aplicada al segundo azul.
  ['--color-on-accent', '--color-accent-strong'],
  ['--color-ok', '--color-bg'],
  ['--color-ok', '--color-surface'],
  ['--color-danger', '--color-bg'],
  ['--color-danger', '--color-surface'],
  // Las pildoras de BaseBadge y la cabecera de BaseTable pintan sobre el fondo
  // de realce. Si estos pares no estuvieran aqui, --color-surface-hover seria un
  // fondo sin medir, que es exactamente como se cuela un texto ilegible.
  ['--color-accent-strong', '--color-surface-hover'],
  ['--color-ok', '--color-surface-hover'],
  ['--color-danger', '--color-surface-hover'],
]

const MINIMO = 4.5

/**
 * Extrae los tokens de un tema. El claro es el bloque `:root` de arriba; el
 * oscuro es el `:root` de dentro del `@media`, que solo redefine lo que cambia,
 * asi que se aplica ENCIMA del claro. Medir el oscuro sobre un objeto vacio
 * daria tokens indefinidos y una prueba que pasa sin medir nada.
 */
function tokens(fuente: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [, nombre, valor] of fuente.matchAll(/(--color-[\w-]+)\s*:\s*(#[0-9a-fA-F]{6})\s*;/g)) {
    out[nombre] = valor
  }
  return out
}

const claro = tokens(css.split('@media')[0]!)
const oscuro = { ...claro, ...tokens(css.slice(css.indexOf('@media'))) }

/** Luminancia relativa, WCAG 2.1 §relative luminance. */
function luminancia(hex: string): number {
  const canales = [1, 3, 5].map((i) => Number.parseInt(hex.slice(i, i + 2), 16) / 255)
  const [r, g, b] = canales.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!
}

function contraste(a: string, b: string): number {
  const [claro_, oscuro_] = [luminancia(a), luminancia(b)].sort((x, y) => y - x)
  return (claro_! + 0.05) / (oscuro_! + 0.05)
}

describe('contraste de los tokens', () => {
  it('la prueba mide colores de verdad', () => {
    // El fallo silencioso de esta prueba es un regex que deja de casar: los
    // temas quedan vacios, no hay par que medir y todo pasa en verde.
    expect(Object.keys(claro).length).toBeGreaterThanOrEqual(9)
    expect(oscuro['--color-accent']).not.toBe(claro['--color-accent'])
  })

  // La referencia de que el calculo esta bien y no solo es autoconsistente:
  // negro sobre blanco es 21:1 exacto, y un color contra si mismo es 1:1.
  it('el calculo coincide con los valores conocidos de WCAG', () => {
    expect(contraste('#000000', '#ffffff')).toBeCloseTo(21, 5)
    expect(contraste('#2f6df6', '#2f6df6')).toBeCloseTo(1, 5)
  })

  for (const [tema, valores] of [
    ['claro', claro],
    ['oscuro', oscuro],
  ] as const) {
    describe(`tema ${tema}`, () => {
      for (const [frente, fondo] of PARES) {
        it(`${frente} sobre ${fondo}`, () => {
          const a = valores[frente]
          const b = valores[fondo]
          expect(a, `${frente} no existe en el tema ${tema}`).toBeDefined()
          expect(b, `${fondo} no existe en el tema ${tema}`).toBeDefined()

          const razon = contraste(a!, b!)
          expect(
            razon,
            `${frente} (${a}) sobre ${fondo} (${b}) da ${razon.toFixed(2)}:1 en tema ${tema}, ` +
              `y hace falta ${MINIMO}:1. Si el color es de marca y no llega, no lo aclares: ` +
              'usa --color-accent-strong para texto y bordes, y deja el de marca para rellenos.',
          ).toBeGreaterThanOrEqual(MINIMO)
        })
      }
    })
  }
})
