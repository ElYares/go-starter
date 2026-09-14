import type { Schemas } from '~/shared/api/generated'

/**
 * La logica de la landing que no necesita a Nuxt: que slug pide una ruta, que
 * codigo HTTP sale de un fallo y que meta etiquetas lleva una pagina. Pura, para
 * probarla en milisegundos; la vista solo la conecta.
 */

/** La portada: lo que se sirve en `/`. La siembra la migracion de content. */
export const SLUG_DE_PORTADA = 'inicio'

// El mismo patron que el contrato y el CHECK de la base. Un slug que no lo
// cumple no puede existir, asi que ni se pregunta: es 404 sin tocar la API.
const SLUG_VALIDO = /^[a-z0-9]+(-[a-z0-9]+)*$/

/**
 * El slug de una ruta de la landing, o `null` si esa ruta no puede ser una
 * pagina. `segmentos` es el `params.slug` del catch-all: ausente o vacio en `/`.
 *
 * `/nosotros/equipo` es null: los slugs no tienen barras, y aceptar el primer
 * segmento serviria la misma pagina en infinitas URLs.
 */
export function slugDeRuta(segmentos: string | string[] | undefined): string | null {
  const partes = (Array.isArray(segmentos) ? segmentos : [segmentos ?? '']).filter((s) => s !== '')

  if (partes.length === 0) return SLUG_DE_PORTADA
  if (partes.length > 1) return null

  const slug = partes[0]!
  return slug.length <= 80 && SLUG_VALIDO.test(slug) ? slug : null
}

/**
 * El codigo con el que responde la landing cuando la API no entrego la pagina.
 *
 * - **404** si la API dijo que no existe, o si rechazo el slug (400): para quien
 *   visita, las dos son "esa pagina no esta"
 * - **503** si no hubo respuesta (`status` ausente: la API esta caida o no se
 *   alcanza) o si lo que respondio es una puerta de enlace sin servicio detras.
 *   Un 200 con una pagina vacia haria que un buscador indexe el hueco
 * - **500** para lo demas: un fallo nuestro no es "vuelve mas tarde"
 */
export function codigoDeFallo(status: number | undefined): 404 | 500 | 503 {
  if (status === undefined || status === 0) return 503
  if (status === 404 || status === 400) return 404
  if (status === 502 || status === 503 || status === 504) return 503
  return 500
}

/**
 * El status que devolvio la API, o `undefined` si no hubo respuesta.
 *
 * NO sirve `error.statusCode` del `useFetch`: Nuxt envuelve el fallo en un
 * NuxtError que trae `statusCode: 500` por omision, tambien cuando la API esta
 * apagada. Con eso una caida saldria como 500 y no como 503. Lo que distingue
 * las dos cosas es la causa: un `FetchError` con `response` si la API respondio,
 * un `TypeError: fetch failed` sin ella si no. Visto en la stack local, 2026-09-14.
 */
export function statusRespondido(error: unknown): number | undefined {
  const causa = (error as { cause?: { response?: { status?: number } } } | null)?.cause
  return causa?.response?.status
}

export interface MetaDePagina {
  title: string
  description?: string
  ogTitle: string
  ogDescription?: string
  ogType: 'website'
  ogUrl?: string
}

/**
 * Las meta etiquetas de una pagina publicada. El titulo SEO manda sobre el
 * titulo, y lo mismo el de Open Graph: es el que se ve en un preview de
 * WhatsApp. Sin descripcion no se inventa una: una description generica igual
 * en todas las paginas es peor que ninguna.
 */
export function metaDePagina(pagina: Schemas['PaginaPublica'], url?: string): MetaDePagina {
  const titulo = pagina.seoTitle || pagina.title
  const descripcion = pagina.seoDescription || undefined

  return {
    title: titulo,
    description: descripcion,
    ogTitle: titulo,
    ogDescription: descripcion,
    ogType: 'website',
    ogUrl: url,
  }
}
