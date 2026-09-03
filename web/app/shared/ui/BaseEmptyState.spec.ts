// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseButton from './BaseButton.vue'
import BaseEmptyState from './BaseEmptyState.vue'

describe('BaseEmptyState', () => {
  it('dice que es esto y que hacer', () => {
    const vacio = mount(BaseEmptyState, {
      props: {
        title: 'Todavia no hay paginas',
        description: 'Crea la primera y apareceria publicada en la landing.',
      },
      slots: { accion: '<BaseButton>Crear pagina</BaseButton>' },
      global: { components: { BaseButton } },
    })

    expect(vacio.text()).toContain('Todavia no hay paginas')
    expect(vacio.text()).toContain('apareceria publicada')
    // La accion es lo que separa un vacio util de un cartel de "no hay nada".
    expect(vacio.find('button').text()).toBe('Crear pagina')
  })

  it('la descripcion es opcional y no deja un parrafo vacio', () => {
    const vacio = mount(BaseEmptyState, { props: { title: 'Sin resultados' } })
    expect(vacio.findAll('p')).toHaveLength(1)
  })
})
