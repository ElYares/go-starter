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
