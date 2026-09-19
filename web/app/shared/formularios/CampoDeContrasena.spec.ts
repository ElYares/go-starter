// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CampoDeContrasena from './CampoDeContrasena.vue'

function montar(props: Record<string, unknown> = {}) {
  return mount(CampoDeContrasena, { props: { modelValue: '', 'onUpdate:modelValue': () => {}, ...props } })
}
const input = (w: ReturnType<typeof montar>) => w.find('input').element as HTMLInputElement

describe('CampoDeContrasena', () => {
  it('empieza oculta y se puede mostrar', async () => {
    const w = montar({ modelValue: 'secreta' })
    expect(input(w).type).toBe('password')
    await w.find('[data-accion="mostrar"]').trigger('click')
    expect(input(w).type).toBe('text')
  })

  it('generar pone una contrasena de 16 y la deja a la vista', async () => {
    const actualizar = vi.fn()
    const w = montar({ generador: true, 'onUpdate:modelValue': actualizar })
    await w.find('[data-accion="generar"]').trigger('click')

    const generada = actualizar.mock.calls[0]?.[0] as string
    expect(generada).toHaveLength(16)
    expect(input(w).type).toBe('text')
  })

  // La contrasena actual no se genera: se escribe la que se tiene.
  it('sin generador no ofrece generar', () => {
    expect(montar().find('[data-accion="generar"]').exists()).toBe(false)
  })

  it('copiar escribe el valor y avisa; si falla, la deja a la vista', async () => {
    const copiar = vi.fn(async () => {})
    const w = montar({ modelValue: 'una temporal bien larga', copiar })
    await w.find('[data-accion="copiar"]').trigger('click')
    expect(copiar).toHaveBeenCalledWith('una temporal bien larga')
    expect(w.find('[data-accion="copiar"]').text()).toBe('Copiada')

    const falla = montar({ modelValue: 'otra', copiar: async () => { throw new Error('sin permiso') } })
    await falla.find('[data-accion="copiar"]').trigger('click')
    await Promise.resolve()
    expect(input(falla).type).toBe('text')
  })

  it('con mostrada empieza a la vista', () => {
    expect(input(montar({ modelValue: 'temporal', mostrada: true })).type).toBe('text')
  })

  it('sin valor no ofrece copiar', () => {
    expect(montar().find('[data-accion="copiar"]').exists()).toBe(false)
  })
})
