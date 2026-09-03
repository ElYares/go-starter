// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import BaseSelect from './BaseSelect.vue'

const OPCIONES = [
  { value: 'borrador', label: 'Borrador' },
  { value: 'publicada', label: 'Publicada' },
  { value: 'archivada', label: 'Archivada', disabled: true },
]

const montados: Array<{ unmount: () => void }> = []

afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(props: Record<string, unknown> = {}) {
  const w = mount(BaseSelect, {
    props: { options: OPCIONES, ...props },
    attachTo: document.body,
  })
  montados.push(w)
  return w
}

describe('BaseSelect', () => {
  it('sin valor muestra el marcador de posicion', () => {
    expect(montar({ placeholder: 'Elegir estado' }).text()).toContain('Elegir estado')
  })

  // Lo que se ve es la ETIQUETA, no el valor crudo. Y se ve SIN haber abierto
  // el panel nunca: es el caso de un formulario precargado desde el servidor,
  // justo el que no se prueba a mano porque a mano siempre se crea algo nuevo.
  it('con valor muestra la etiqueta sin haber abierto el panel', () => {
    const select = montar({ modelValue: 'publicada', placeholder: 'Elegir estado' })
    expect(select.text()).toContain('Publicada')
    expect(select.text()).not.toContain('Elegir estado')
  })

  it('el disparador es un boton con rol de combobox', () => {
    const disparador = montar().find('[role="combobox"]')
    expect(disparador.exists()).toBe(true)
    expect(disparador.attributes('aria-expanded')).toBe('false')
  })

  it('abre el panel con teclado y lista las opciones', async () => {
    const select = montar()
    await select.find('[role="combobox"]').trigger('keydown', { key: 'Enter' })
    await nextTick()

    const opciones = [...document.querySelectorAll('[role="option"]')].map((o) =>
      o.textContent?.trim(),
    )
    expect(opciones).toEqual(['Borrador', 'Publicada', 'Archivada'])
  })

  it('una opcion deshabilitada se anuncia como tal', async () => {
    const select = montar()
    await select.find('[role="combobox"]').trigger('keydown', { key: 'Enter' })
    await nextTick()

    const archivada = [...document.querySelectorAll('[role="option"]')].find(
      (o) => o.textContent?.trim() === 'Archivada',
    )
    expect(archivada?.getAttribute('aria-disabled')).toBe('true')
  })

  // Igual que en BaseInput: el borde rojo no es lo que informa.
  it('invalido se anuncia y acepta el cableado de BaseField', () => {
    const select = montar({ id: 'estado', invalid: true, describedBy: 'estado-error' })
    const disparador = select.find('[role="combobox"]')
    expect(disparador.attributes('aria-invalid')).toBe('true')
    expect(disparador.attributes('aria-describedby')).toBe('estado-error')
    expect(disparador.attributes('id')).toBe('estado')
  })

  it('deshabilitado no abre', async () => {
    const select = montar({ disabled: true })
    await select.find('[role="combobox"]').trigger('keydown', { key: 'Enter' })
    await nextTick()
    expect(document.querySelectorAll('[role="option"]')).toHaveLength(0)
  })
})
