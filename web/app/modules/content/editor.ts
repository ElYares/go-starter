import type { Schemas } from '~/shared/api/generated'
import { valorInicial, type Esquema } from '~/shared/formularios/esquema'

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
 * Un bloque nuevo del tipo pedido, con un id que no usa ningun otro de la
 * pagina. El servidor rechaza ids repetidos, y el editor los usa como clave.
 */
export function nuevoBloque(tipo: string, esquema: Esquema, existentes: readonly Bloque[]): Bloque {
  const usados = new Set(existentes.map((b) => b.id))
  let n = 1
  while (usados.has(`${tipo}-${n}`)) n++
  return { id: `${tipo}-${n}`, type: tipo, props: valorInicial(esquema) as Record<string, unknown> }
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
