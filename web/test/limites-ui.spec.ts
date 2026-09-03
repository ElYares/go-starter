import { readFileSync, readdirSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// El gemelo de api/internal/app/limites_test.go, del lado del frontend.
//
// La Decision 005 dice que ninguna vista importa Reka UI directamente. Esa regla
// es lo unico que hace reversible la eleccion de libreria: si se erosiona, el dia
// que Reka cambie de API o se abandone no son diez archivos de shared/ui/, son
// cincuenta vistas. Y se erosiona sola, porque importar el primitivo directo
// siempre es mas rapido que agregarle una prop al Base*.
//
// Como en el lado de Go, se lee el texto de los archivos y no el grafo de
// modulos: para saber que importa un archivo no hace falta resolver nada, y
// evita arrastrar un analizador de TypeScript a las pruebas.

const raizApp = fileURLToPath(new URL('../app', import.meta.url))

// Cubre las tres formas de traerse la libreria: el import estatico, el dinamico
// y el require de un archivo .ts suelto. Solo la primera es probable; las otras
// dos son justo por donde se cuela quien ya sabe que hay una prueba.
const IMPORTA_REKA = /(?:from\s*|import\s*\(\s*|require\s*\(\s*)['"]reka-ui['"]/

function archivosDeCodigo(dir: string): string[] {
  const out: string[] = []
  for (const entrada of readdirSync(dir, { withFileTypes: true })) {
    const ruta = join(dir, entrada.name)
    if (entrada.isDirectory()) {
      if (entrada.name === 'node_modules' || entrada.name === 'generated') continue
      out.push(...archivosDeCodigo(ruta))
      continue
    }
    if (/\.(vue|ts)$/.test(entrada.name)) out.push(ruta)
  }
  return out
}

describe('limites del frontend', () => {
  const archivos = archivosDeCodigo(raizApp).map((ruta) => ({
    ruta: relative(raizApp, ruta),
    fuente: readFileSync(ruta, 'utf8'),
  }))

  it('lee archivos de verdad', () => {
    // Sin esto, un recorrido que no encuentra nada haria pasar la regla de abajo
    // por vacuidad, y el reporte diria verde sin haber mirado un solo archivo.
    // Es exactamente el fallo que el lado de Go ya se cuida de tener.
    expect(archivos.length).toBeGreaterThan(5)
  })

  it('mantiene a Reka UI dentro de shared/ui/', () => {
    const infractores = archivos
      .filter(({ ruta }) => !ruta.startsWith('shared/ui/'))
      .filter(({ fuente }) => IMPORTA_REKA.test(fuente))
      .map(({ ruta }) => ruta)

    expect(
      infractores,
      `estos archivos importan Reka UI fuera de shared/ui/:\n  ${infractores.join('\n  ')}\n` +
        'envuelve el primitivo en un componente de shared/ui/ y usa ese',
    ).toEqual([])
  })

  it('shared/ui/ es quien la usa, y por eso la regla no es una carpeta muerta', () => {
    // La regla de arriba pasa sola si nadie usa Reka en ningun lado. Esta afirma
    // el otro extremo: la envoltura existe de verdad.
    const enShared = archivos.filter(
      ({ ruta, fuente }) => ruta.startsWith('shared/ui/') && IMPORTA_REKA.test(fuente),
    )
    expect(enShared.length).toBeGreaterThan(0)
  })
})
