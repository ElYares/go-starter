/**
 * La forma de las `props` de cada tipo de bloque, del lado del frontend.
 *
 * El contrato declara `props` como objeto libre porque su forma depende del
 * tipo; la valida el servidor al guardar contra el JSON Schema del catalogo
 * (`api/internal/modules/content/bloques/<tipo>.json`). Estas interfaces son su
 * espejo escrito a mano, y por eso los componentes no confian en ellas: todo lo
 * opcional en el esquema se comprueba antes de pintarlo.
 */

export interface HeroContenido {
  title: string
  subtitle?: string
  cta?: { label: string; href: string }
}

export interface FeaturesContenido {
  title?: string
  items: { title: string; text?: string }[]
}

export interface TextoContenido {
  title?: string
  body: string
}
