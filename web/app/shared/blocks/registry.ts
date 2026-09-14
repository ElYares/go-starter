import type { Component } from 'vue'
import BloqueDesconocido from './BloqueDesconocido.vue'
import FeaturesBlock from './FeaturesBlock.vue'
import HeroBlock from './HeroBlock.vue'
import TextoBlock from './TextoBlock.vue'

/**
 * El registro de tipos de bloque. Un tipo son dos cosas que viajan juntas: el
 * JSON Schema en `api/internal/modules/content/bloques/<tipo>.json` y su
 * componente aqui. `registry.spec.ts` compara las dos listas y falla si una
 * tiene un tipo que la otra no.
 *
 * Es lo que un fork reescribe para cambiar el lenguaje visual del sitio. El
 * editor (CU-004) leera este mismo registro para ofrecer "agregar bloque".
 *
 * Importaciones directas y no `() => import()`: son tres componentes chicos que
 * la portada usa siempre, y partirlos cuesta una peticion por bloque sin
 * ahorrar nada.
 */
export interface TipoDeBloque {
  component: Component
  label: string
}

export const bloques: Record<string, TipoDeBloque> = {
  hero: { component: HeroBlock, label: 'Portada' },
  features: { component: FeaturesBlock, label: 'Caracteristicas' },
  texto: { component: TextoBlock, label: 'Texto' },
}

/**
 * `null` para un tipo que no esta. `Object.hasOwn` y no `bloques[tipo]`: un
 * bloque con `type: "constructor"` o `"toString"` encontraria algo en el
 * prototipo y lo intentaria renderizar como componente.
 */
export function resolverBloque(tipo: string): TipoDeBloque | null {
  return Object.hasOwn(bloques, tipo) ? bloques[tipo]! : null
}

export { BloqueDesconocido }
