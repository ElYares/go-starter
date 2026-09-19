/**
 * Si un `href` que viene del contenido o de la configuracion se puede pintar.
 *
 * Es el mismo patron que los esquemas del api (`hero.json`, `site.nav.json`,
 * `site.footer.json`): una ruta del sitio o `https://`. El api ya valida al
 * guardar, pero la landing lo repite a la defensiva: una fila escrita a mano en
 * la base no pasa por el esquema, y lo que sale de aqui es un enlace en la
 * portada con el dominio del sitio.
 *
 * Lo que rechaza y por que:
 *
 * - `javascript:` y `data:` son un XSS
 * - `//otro.com` empieza por `/` pero lleva a otro sitio
 * - `/\otro.com` y una barra seguida de tabulador o salto de linea: el
 *   navegador los trata igual que `//`, y son el mismo redireccionamiento
 *   abierto con otra ropa
 */
export const HREF_SEGURO = /^(\/([^/\\\s]|$)|https:\/\/)/

export const esHrefSeguro = (href: unknown): href is string => typeof href === 'string' && HREF_SEGURO.test(href)
