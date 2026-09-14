/**
 * El catalogo de bloques del api, visto desde el navegador: el MISMO JSON Schema
 * que valida las props al guardar (`api/internal/modules/content/bloques/`).
 *
 * Se importa en el build y no se pide a un endpoint: el catalogo cambia con el
 * codigo, igual que los componentes de bloque, asi que viaja con el despliegue.
 * El editor arma sus formularios desde aqui, y por eso un fork que agrega un
 * tipo no escribe formulario: escribe el esquema y el componente.
 *
 * La ruta relativa es la misma en el checkout de CI y dentro del contenedor de
 * web, que monta esa carpeta en `/api/...` (compose.yaml).
 */

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

const archivos = import.meta.glob<Esquema>('../../../../api/internal/modules/content/bloques/*.json', {
  eager: true,
  import: 'default',
})

export const esquemas: Record<string, Esquema> = Object.fromEntries(
  Object.entries(archivos).map(([ruta, esquema]) => [ruta.slice(ruta.lastIndexOf('/') + 1, -'.json'.length), esquema]),
)
