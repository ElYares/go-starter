import type { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import type { Esquema } from '~/shared/blocks/catalogo'

// La logica del editor de paginas, sin Vue ni Nuxt: que se crea al agregar un
// bloque, como se mueve, que errores tocan a que campo y cuando hay cambios.

export type Bloque = Schemas['Bloque']

/** Lo que se edita: todo lo que viaja en un guardado. */
export interface Borrador {
  slug: string
  title: string
  seoTitle: string
  seoDescription: string
  note: string
  blocks: Bloque[]
}

export function borradorDe(p: Schemas['Pagina']): Borrador {
  return {
    slug: p.slug,
    title: p.title,
    seoTitle: p.seoTitle ?? '',
    seoDescription: p.seoDescription ?? '',
    // La nota es de cada guardado, no de la pagina: se empieza vacia.
    note: '',
    // Copia profunda: el formulario muta el borrador, y compartir objetos con
    // la pagina leida haria que "hay cambios" nunca fuera verdad.
    //
    // Por JSON y no con structuredClone: la pagina llega envuelta en un proxy
    // reactivo de Vue, y structuredClone lanza DataCloneError con un proxy. Los
    // bloques son JSON por contrato, asi que no se pierde nada.
    blocks: JSON.parse(JSON.stringify(p.blocks)) as Bloque[],
  }
}

/**
 * El cuerpo del PUT. Los opcionales vacios van como null: un `seoTitle: ""`
 * pisaria el titulo en las meta etiquetas con una cadena vacia.
 */
export function cuerpoDeGuardado(b: Borrador): Schemas['PaginaModificacion'] {
  const opcional = (v: string) => (v.trim() === '' ? null : v)
  return {
    slug: b.slug,
    title: b.title,
    seoTitle: opcional(b.seoTitle),
    seoDescription: opcional(b.seoDescription),
    note: opcional(b.note),
    blocks: b.blocks,
  }
}

export function hayCambios(pagina: Schemas['Pagina'], b: Borrador): boolean {
  const { note: _, ...resto } = borradorDe(pagina)
  const { note, ...actual } = b
  return note.trim() !== '' || JSON.stringify(resto) !== JSON.stringify(actual)
}

export type EstadoDePublicacion = 'sin-publicar' | 'al-dia' | 'cambios-sin-publicar'

export function estadoDePublicacion(p: Schemas['Pagina']): EstadoDePublicacion {
  if (p.publishedVersionId === null) return 'sin-publicar'
  return p.publishedVersionId === p.draftVersionId ? 'al-dia' : 'cambios-sin-publicar'
}

/**
 * El valor con el que nace un campo: lo minimo para que el esquema tenga donde
 * poner lo obligatorio. Lo opcional no se crea: un `subtitle: ""` que nadie
 * escribio no es lo mismo que no tener bajada.
 */
export function valorInicial(esquema: Esquema): unknown {
  switch (esquema.type) {
    case 'string':
      return ''
    case 'object': {
      const obj: Record<string, unknown> = {}
      for (const clave of esquema.required ?? []) {
        const prop = esquema.properties?.[clave]
        if (prop) obj[clave] = valorInicial(prop)
      }
      return obj
    }
    case 'array':
      return Array.from({ length: esquema.minItems ?? 0 }, () => (esquema.items ? valorInicial(esquema.items) : null))
    default:
      return null
  }
}

/**
 * Un bloque nuevo del tipo pedido, con un id que no usa ningun otro de la
 * pagina. El servidor rechaza ids repetidos, y el editor los usa como clave.
 */
export function nuevoBloque(tipo: string, esquema: Esquema, existentes: readonly Bloque[]): Bloque {
  const usados = new Set(existentes.map((b) => b.id))
  let n = 1
  while (usados.has(`${tipo}-${n}`)) n++
  return { id: `${tipo}-${n}`, type: tipo, props: valorInicial(esquema) as Record<string, unknown> }
}

/** Mueve un elemento una posicion. Fuera de rango, devuelve la lista igual. */
export function mover<T>(lista: readonly T[], i: number, delta: -1 | 1): T[] {
  const j = i + delta
  if (i < 0 || i >= lista.length || j < 0 || j >= lista.length) return [...lista]
  const copia = [...lista]
  ;[copia[i], copia[j]] = [copia[j]!, copia[i]!]
  return copia
}

/**
 * Los errores de un 400, por campo. La ruta es la del api
 * (`blocks[2].props.items[0].title`), que es la misma con la que el formulario
 * nombra sus campos. Si un campo trae varios, se queda el primero: un campo
 * muestra un mensaje, no una lista.
 */
export function erroresPorCampo(fallo: ApiError | null | undefined): Record<string, string> {
  const out: Record<string, string> = {}
  for (const e of fallo?.errors ?? []) {
    out[e.field] ??= e.message
  }
  return out
}

/**
 * Si un bloque tiene algun error. El prefijo lleva el corchete de cierre, y eso
 * es lo que evita que `blocks[1]` se quede con los errores de `blocks[10]`.
 */
export function bloqueConError(errores: Record<string, string>, indice: number): boolean {
  const base = `blocks[${indice}]`
  return Object.keys(errores).some((c) => c.startsWith(base))
}

/** Un slug a partir de un titulo, para no hacer escribir los dos. */
export function sugerirSlug(titulo: string): string {
  return titulo
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80)
    .replace(/-+$/, '')
}
