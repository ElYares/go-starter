// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseInput from './BaseInput.vue'

describe('BaseInput', () => {
  it('va y viene con v-model', async () => {
    const entrada = mount(BaseInput, { props: { modelValue: 'hola' } })
    expect(entrada.element.value).toBe('hola')

    await entrada.setValue('adios')
    expect(entrada.emitted('update:modelValue')).toEqual([['adios']])
  })

  // El borde rojo no es la parte importante. aria-invalid es lo que hace que un
  // lector de pantalla diga "no valido" al entrar al campo.
  it('invalido se anuncia, no solo se pinta', () => {
    expect(mount(BaseInput).attributes('aria-invalid')).toBeUndefined()
    expect(mount(BaseInput, { props: { invalid: true } }).attributes('aria-invalid')).toBe('true')
  })

  it('acepta el id y la descripcion que le pasa BaseField', () => {
    const entrada = mount(BaseInput, { props: { id: 'x1', describedBy: 'x1-error' } })
    expect(entrada.attributes('id')).toBe('x1')
    expect(entrada.attributes('aria-describedby')).toBe('x1-error')
  })
})
