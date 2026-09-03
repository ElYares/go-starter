// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseButton from './BaseButton.vue'

describe('BaseButton', () => {
  it('pinta su contenido y emite el clic', async () => {
    const boton = mount(BaseButton, { slots: { default: 'Guardar' } })
    expect(boton.text()).toBe('Guardar')

    await boton.trigger('click')
    expect(boton.emitted('click')).toHaveLength(1)
  })

  // La razon de existir del defecto explicito: dentro de un <form>, un <button>
  // sin type envia el formulario. Un boton "Cancelar" al lado de "Guardar"
  // termina guardando, y el sintoma aparece en produccion.
  it('por omision es type="button" y no envia formularios', () => {
    expect(mount(BaseButton).attributes('type')).toBe('button')
    expect(mount(BaseButton, { props: { type: 'submit' } }).attributes('type')).toBe('submit')
  })

  it('deshabilitado no llega a emitir el clic', async () => {
    const boton = mount(BaseButton, { props: { disabled: true } })
    await boton.trigger('click')
    expect(boton.emitted('click')).toBeUndefined()
  })

  // Cargando tiene que bloquear igual que deshabilitado: si no, el doble clic
  // impaciente manda la peticion dos veces. Y aria-busy es lo que se lo dice a
  // quien no ve la ruleta.
  it('cargando bloquea el clic y lo anuncia', async () => {
    const boton = mount(BaseButton, { props: { loading: true } })
    expect(boton.attributes('aria-busy')).toBe('true')
    expect(boton.attributes('disabled')).toBeDefined()

    await boton.trigger('click')
    expect(boton.emitted('click')).toBeUndefined()
  })

  it('la variante cambia la clase, no el marcado', () => {
    expect(mount(BaseButton, { props: { variant: 'danger' } }).classes()).toContain('v-danger')
    expect(mount(BaseButton, { props: { size: 'sm' } }).classes()).toContain('t-sm')
  })
})
