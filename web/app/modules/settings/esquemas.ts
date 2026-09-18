/**
 * Los esquemas de las claves de settings, vistos desde el navegador: los MISMOS
 * JSON Schema que valida el api al guardar (`api/internal/modules/settings/esquemas/`).
 *
 * Se importan en el build, como el catalogo de bloques (`shared/blocks/catalogo.ts`):
 * cambian con el codigo y viajan con el despliegue. El editor arma el formulario
 * desde aqui, asi que agregar una clave no es escribir un formulario: es
 * escribir su esquema. Ver la Decision 022.
 *
 * Llegan con sus `$ref` ya resueltos: el formulario no sabe de `$defs`.
 */
import { resolverReferencias, type Esquema } from '~/shared/formularios/esquema'

const archivos = import.meta.glob<Esquema>('../../../../api/internal/modules/settings/esquemas/*.json', {
  eager: true,
  import: 'default',
})

export const esquemas: Record<string, Esquema> = Object.fromEntries(
  Object.entries(archivos).map(([ruta, esquema]) => [
    ruta.slice(ruta.lastIndexOf('/') + 1, -'.json'.length),
    resolverReferencias(esquema),
  ]),
)
