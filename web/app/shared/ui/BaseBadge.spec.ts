// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseBadge from './BaseBadge.vue'

describe('BaseBadge', () => {
  it('pinta su contenido con la variante pedida', () => {
    const pildora = mount(BaseBadge, {
      props: { variant: 'ok' },
      slots: { default: 'Publicada' },
    })
    expect(pildora.text()).toBe('Publicada')
    expect(pildora.classes()).toContain('v-ok')
  })

  // El color no es informacion: una pildora roja no dice "error" a quien no
  // distingue rojo de verde, ni a un lector de pantalla. Cuando el estado solo
  // viaja en el color, srLabel lo pone en palabras.
  it('srLabel pone en palabras lo que solo dice el color', () => {
    const pildora = mount(BaseBadge, {
      props: { variant: 'danger', srLabel: 'Estado' },
      slots: { default: 'Caida' },
    })
    expect(pildora.text()).toContain('Estado:')
    // Oculto a la vista, no al lector: nada de display:none ni aria-hidden.
    const oculto = pildora.find('.solo-lector')
    expect(oculto.attributes('aria-hidden')).toBeUndefined()
  })

  it('sin srLabel no mete texto de mas', () => {
    const pildora = mount(BaseBadge, { slots: { default: 'Borrador' } })
    expect(pildora.text()).toBe('Borrador')
    expect(pildora.classes()).toContain('v-neutral')
  })
})
