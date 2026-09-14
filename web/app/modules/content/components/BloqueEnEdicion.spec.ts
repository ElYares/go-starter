// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { esquemas } from '~/shared/blocks/catalogo'
import BloqueEnEdicion from './BloqueEnEdicion.vue'

const base = { indice: 1, total: 3, errores: {} as Record<string, string> }

describe('BloqueEnEdicion', () => {
  it('pinta el formulario de su tipo y avisa al cambiar, conservando id y tipo', async () => {
    const w = mount(BloqueEnEdicion, {
      props: { ...base, bloque: { id: 'b2', type: 'texto', props: { body: 'Hola' } }, esquema: esquemas.texto, etiqueta: 'Texto' },
    })
    await w.find('[data-campo="blocks[1].props.body"]').setValue('Adios')
    expect(w.emitted('update:bloque')!.at(-1)![0]).toEqual({ id: 'b2', type: 'texto', props: { body: 'Adios' } })
  })

  it('sube, baja y se quita, sin subir el primero ni bajar el ultimo', async () => {
    const primero = mount(BloqueEnEdicion, {
      props: { ...base, indice: 0, bloque: { id: 'a', type: 'texto', props: { body: 'x' } }, esquema: esquemas.texto },
    })
    const boton = (w: typeof primero, texto: string) => w.findAll('button').find((b) => b.text() === texto)!
    expect(boton(primero, 'Subir').attributes('disabled')).toBeDefined()
    await boton(primero, 'Bajar').trigger('click')
    await boton(primero, 'Quitar').trigger('click')
    expect(primero.emitted('bajar')).toHaveLength(1)
    expect(primero.emitted('quitar')).toHaveLength(1)

    const ultimo = mount(BloqueEnEdicion, {
      props: { ...base, indice: 2, bloque: { id: 'c', type: 'texto', props: { body: 'x' } }, esquema: esquemas.texto },
    })
    expect(boton(ultimo, 'Bajar').attributes('disabled')).toBeDefined()
  })

  // CU-004 criterio 7 y E3.
  it('un tipo desconocido se muestra como no reconocido, con su JSON, y se puede quitar', async () => {
    const w = mount(BloqueEnEdicion, {
      props: { ...base, bloque: { id: 'x', type: 'carrusel', props: { fotos: ['a.jpg'] } } },
    })
    expect(w.text()).toContain('Bloque no reconocido')
    expect(w.text()).toContain('carrusel')
    expect(JSON.parse(w.find('pre').text())).toEqual({ fotos: ['a.jpg'] })
    expect(w.find('input, textarea').exists()).toBe(false)

    await w.findAll('button').find((b) => b.text() === 'Quitar')!.trigger('click')
    expect(w.emitted('quitar')).toHaveLength(1)
    expect(w.emitted('update:bloque')).toBeUndefined()
  })

  it('se marca con errores por los de sus props y muestra los del propio bloque', () => {
    const w = mount(BloqueEnEdicion, {
      props: {
        ...base,
        bloque: { id: 'b2', type: 'texto', props: { body: '' } },
        esquema: esquemas.texto,
        errores: { 'blocks[1].props.body': 'No puede estar vacio', 'blocks[1].id': 'Ya lo usa el bloque 0' },
      },
    })
    expect(w.find('article').classes()).toContain('con-error')
    expect(w.text()).toContain('Ya lo usa el bloque 0')
    expect(w.text()).toContain('No puede estar vacio')
  })

  it('sin permiso de escritura no ofrece acciones', () => {
    const w = mount(BloqueEnEdicion, {
      props: { ...base, bloque: { id: 'b', type: 'texto', props: { body: 'x' } }, esquema: esquemas.texto, deshabilitado: true },
    })
    expect(w.findAll('button')).toHaveLength(0)
  })
})
