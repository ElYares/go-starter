import type { Schemas } from '~/shared/api/generated'

/**
 * El marco de la landing —marca, menu y pie— a partir de `/public/settings`.
 * Puro, como `pagina.ts`: la vista solo lo conecta.
 *
 * El api ya valida cada clave contra su esquema (HU-012), pero aqui se lee a la
 * defensiva: una fila escrita a mano en la base no pasa por el esquema, y lo que
 * sale de aqui termina en un `href` de todas las paginas del sitio.
 */

export interface Enlace {
  label: string
  href: string
}

export interface Sitio {
  nombre: string
  lema?: string
  /** El id de la imagen del logo. La URL la arma quien la pinta. */
  logo?: string
  nav: Enlace[]
  pie: { texto?: string; enlaces: Enlace[] }
}

// El mismo patron que los esquemas de settings: una ruta del sitio o https.
// `//otro.com` empieza por `/` y lleva a otro sitio; `javascript:` es un XSS.
const HREF_SEGURO = /^(\/([^/]|$)|https:\/\/)/

const texto = (v: unknown): string | undefined => (typeof v === 'string' && v.trim() !== '' ? v : undefined)

function enlaces(v: unknown): Enlace[] {
  if (!Array.isArray(v)) return []
  return v.flatMap((e) => {
    const label = texto((e as Enlace | null)?.label)
    const href = texto((e as Enlace | null)?.href)
    return label && href && HREF_SEGURO.test(href) ? [{ label, href }] : []
  })
}

/**
 * El sitio, o `null` si no hay con que pintar la cabecera: sin nombre no hay
 * marca, y una cabecera vacia es peor que ninguna.
 */
export function sitioDe(mapa: Schemas['SettingsMap'] | null | undefined): Sitio | null {
  const marca = mapa?.['site.brand'] as Record<string, unknown> | undefined
  const nombre = texto(marca?.name)
  if (!nombre) return null

  const pie = mapa?.['site.footer'] as Record<string, unknown> | undefined
  return {
    nombre,
    lema: texto(marca?.tagline),
    logo: texto(marca?.logo),
    nav: enlaces(mapa?.['site.nav']),
    pie: { texto: texto(pie?.text), enlaces: enlaces(pie?.links) },
  }
}
