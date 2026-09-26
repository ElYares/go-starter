// El valor de una clave de configuracion, dicho para una persona y no para el
// api: "Nombre: go-starter" en vez de {"name":"go-starter"}, y el color como
// una muestra. Sale del mismo JSON Schema que arma el editor (Decision 022),
// asi que una clave nueva se resume sola: con los `title` de su esquema.
//
// Sin Vue: se prueba en node.
import { esColor, FORMATO_COLOR, FORMATO_MEDIO, type Esquema } from '~/shared/formularios/esquema'

/** Una linea del resumen. `etiqueta` falta cuando el valor entero es una lista. */
export type Linea =
  | { tipo: 'texto'; etiqueta?: string; texto: string }
  | { tipo: 'color'; etiqueta?: string; color: string }
  | { tipo: 'vacio'; etiqueta?: string; texto: string }
  | { tipo: 'json'; etiqueta?: string; texto: string }

/** El tope de una linea: un texto de quinientos caracteres no puede estirar la tarjeta. */
export const LARGO_MAXIMO = 80

/** Cuantos elementos de una lista se nombran antes de decir "y N mas". */
const NOMBRADOS = 3

const recortar = (texto: string) => (texto.length > LARGO_MAXIMO ? `${texto.slice(0, LARGO_MAXIMO - 1)}…` : texto)

/**
 * Las lineas con que se resume `valor`. Un objeto da una linea por propiedad
 * del esquema, en su orden y con su `title`; cualquier otra cosa, una sola.
 *
 * Sin esquema no hay a quien preguntarle que significa cada campo: se muestra
 * el JSON, recortado.
 */
export function resumir(esquema: Esquema | undefined, valor: unknown): Linea[] {
  if (!esquema) return [{ tipo: 'json', texto: recortar(JSON.stringify(valor) ?? '') }]

  if (esquema.type === 'object' && esquema.properties) {
    const objeto = (valor && typeof valor === 'object' ? valor : {}) as Record<string, unknown>
    return Object.entries(esquema.properties).map(([clave, sub]) => linea(sub.title ?? clave, sub, objeto[clave]))
  }
  return [linea(undefined, esquema, valor)]
}

function linea(etiqueta: string | undefined, esquema: Esquema, valor: unknown): Linea {
  if (valor === undefined || valor === null || valor === '') {
    return { tipo: 'vacio', etiqueta, texto: esquema.format === FORMATO_MEDIO ? 'Sin imagen' : 'Sin definir' }
  }

  if (esquema.format === FORMATO_COLOR && esColor(valor)) return { tipo: 'color', etiqueta, color: valor }
  if (esquema.format === FORMATO_MEDIO) return { tipo: 'texto', etiqueta, texto: 'Imagen cargada' }
  if (typeof valor === 'string') return { tipo: 'texto', etiqueta, texto: recortar(valor) }
  if (typeof valor === 'number') return { tipo: 'texto', etiqueta, texto: String(valor) }
  if (typeof valor === 'boolean') return { tipo: 'texto', etiqueta, texto: valor ? 'Si' : 'No' }

  if (Array.isArray(valor)) {
    if (valor.length === 0) return { tipo: 'vacio', etiqueta, texto: 'Ninguno' }
    const nombres = valor.slice(0, NOMBRADOS).map((el) => nombreDe(esquema.items, el))
    const resto = valor.length - nombres.length
    const texto = resto > 0 ? `${nombres.join(', ')} y ${resto} mas` : nombres.join(', ')
    return { tipo: 'texto', etiqueta, texto: recortar(texto) }
  }

  return { tipo: 'json', etiqueta, texto: recortar(JSON.stringify(valor)) }
}

/**
 * Como se nombra un elemento de una lista: el primer texto de sus propiedades,
 * en el orden del esquema. En un enlace es `label` ("Inicio"), no `href`.
 */
function nombreDe(esquema: Esquema | undefined, elemento: unknown): string {
  if (typeof elemento === 'string') return elemento
  if (elemento && typeof elemento === 'object' && esquema?.properties) {
    const objeto = elemento as Record<string, unknown>
    for (const clave of Object.keys(esquema.properties)) {
      const v = objeto[clave]
      if (typeof v === 'string' && v !== '') return v
    }
  }
  return JSON.stringify(elemento)
}

const MESES = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']

/**
 * "18 sep 2026", del dia que trae la marca de tiempo del api. Se lee de la
 * cadena y no con `Date`: asi no depende de la zona del navegador, y dice el
 * mismo dia que decia la tabla de antes (`slice(0, 10)`).
 */
export function fecha(iso: string): string {
  const [anio, mes, dia] = iso.slice(0, 10).split('-').map(Number)
  const nombre = MESES[(mes ?? 0) - 1]
  if (!anio || !nombre || !dia) return iso.slice(0, 10)
  return `${dia} ${nombre} ${anio}`
}
