// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseField from './BaseField.vue'
import BaseInput from './BaseInput.vue'

// El campo completo, montado como se usa de verdad: la etiqueta tiene que
// apuntar al control real, no a un id inventado.
function montar(props: Record<string, unknown>) {
  return mount(BaseField, {
    props: { label: 'Correo', ...props },
    slots: {
      default: `<BaseInput :id="params.id" :described-by="params.describedBy" :invalid="params.invalid" />`,
    },
    global: { components: { BaseInput } },
  })
}

describe('BaseField', () => {
  it('la etiqueta apunta al control que envuelve', () => {
    const campo = montar({})
    const idEtiqueta = campo.find('label').attributes('for')
    const idEntrada = campo.find('input').attributes('id')

    expect(idEtiqueta).toBeTruthy()
    expect(idEtiqueta).toBe(idEntrada)
  })

  // Dos campos en la misma pantalla no pueden compartir id: si lo hacen, hacer
  // clic en la segunda etiqueta enfoca el primer input.
  //
  // Los dos van en UN solo montaje a proposito. useId() de Vue cuenta por
  // instancia de aplicacion, asi que dos mount() separados devuelven 'v-0' los
  // dos y la prueba fallaria sin que el componente tenga nada malo. Lo que hay
  // que afirmar es lo que pasa en una pantalla real: una app, varios campos.
  it('dos campos de la misma pantalla no comparten id', () => {
    const pantalla = mount(
      {
        components: { BaseField, BaseInput },
        template: `
          <form>
            <BaseField label="Correo" v-slot="p"><BaseInput :id="p.id" /></BaseField>
            <BaseField label="Clave" v-slot="p"><BaseInput :id="p.id" /></BaseField>
          </form>`,
      },
      { global: { components: { BaseField, BaseInput } } },
    )

    const ids = pantalla.findAll('input').map((i) => i.attributes('id'))
    const fors = pantalla.findAll('label').map((l) => l.attributes('for'))

    expect(ids).toHaveLength(2)
    expect(new Set(ids).size).toBe(2)
    expect(fors).toEqual(ids)
  })

  it('el error se anuncia y queda enlazado al control', () => {
    const campo = montar({ error: 'Ese correo ya existe' })
    const error = campo.find('[role="alert"]')

    expect(error.text()).toBe('Ese correo ya existe')
    expect(campo.find('input').attributes('aria-describedby')).toBe(error.attributes('id'))
    expect(campo.find('input').attributes('aria-invalid')).toBe('true')
  })

  it('la pista se enlaza igual cuando no hay error', () => {
    const campo = montar({ hint: 'Lo usamos para avisarte' })
    expect(campo.find('input').attributes('aria-describedby')).toBe(
      campo.find('p').attributes('id'),
    )
  })

  // Sin esto, aria-describedby apuntaria a un id que no existe en el DOM, y el
  // lector de pantalla no lee nada donde deberia leer algo.
  it('sin pista ni error no deja una referencia colgando', () => {
    expect(montar({}).find('input').attributes('aria-describedby')).toBeUndefined()
  })

  it('el error gana a la pista', () => {
    const campo = montar({ hint: 'una pista', error: 'un error' })
    expect(campo.text()).toContain('un error')
    expect(campo.text()).not.toContain('una pista')
  })
})
