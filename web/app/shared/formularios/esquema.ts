import type { ApiError } from '~/shared/api/errors'

// Lo que los formularios generados del JSON Schema necesitan, sin Vue ni Nuxt.
// Lo usan dos modulos: el editor de paginas (los bloques) y la configuracion
// del sitio (las claves de settings). Ver la Decision 022.

/** Lo que el editor entiende de un JSON Schema. Lo demas se ignora. */
export interface Esquema {
  title?: string
  description?: string
  type?: string
  properties?: Record<string, Esquema>
  required?: string[]
  items?: Esquema
  minLength?: number
  maxLength?: number
  minItems?: number
  maxItems?: number
  pattern?: string
  /**
   * `media-id` es el id de una imagen subida: se edita con un campo de subida,
   * no escribiendo el uuid. Los demas formatos se ignoran.
   */
  format?: string
  $ref?: string
  $defs?: Record<string, Esquema>
}

/** El `format` de un campo que guarda el id de una imagen (settings/esquemas). */
export const FORMATO_MEDIO = 'media-id'

/** El tope de una imagen, el mismo que aplica el api (media.MaxBytes). */
export const MAX_BYTES_MEDIO = 5 * 1024 * 1024

/**
 * Donde se sirven los bytes de una imagen. El dashboard es SPA, asi que la ruta
 * es la del navegador; es la misma que devuelve el api en `Medio.url`.
 */
export const urlDeMedio = (id: string) => `/api/v1/public/media/${id}`

/**
 * Devuelve el esquema con sus `$ref` locales (`#/$defs/enlace`) sustituidos por
 * lo que nombran, para que el formulario no tenga que resolverlos al pintar.
 *
 * Solo entiende referencias a `$defs` del mismo archivo, que es lo que usan los
 * esquemas del starter. Una que no encuentra, o una que se nombra a si misma,
 * se deja como esta: el formulario la pinta como "no se puede editar" y
 * conserva el valor, en vez de colgarse o inventar un campo.
 */
export function resolverReferencias(raiz: Esquema): Esquema {
  const defs = raiz.$defs ?? {}

  function resolver(e: Esquema, visitando: ReadonlySet<string>): Esquema {
    if (e.$ref) {
      const nombre = e.$ref.startsWith('#/$defs/') ? e.$ref.slice('#/$defs/'.length) : undefined
      const destino = nombre ? defs[nombre] : undefined
      if (!nombre || !destino || visitando.has(nombre)) return e
      const { $ref: _, ...resto } = e
      return resolver({ ...destino, ...resto }, new Set([...visitando, nombre]))
    }
    const out: Esquema = { ...e }
    if (e.properties) {
      out.properties = Object.fromEntries(Object.entries(e.properties).map(([k, v]) => [k, resolver(v, visitando)]))
    }
    if (e.items) out.items = resolver(e.items, visitando)
    return out
  }

  return resolver(raiz, new Set())
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
