// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { esquemas } from '~/shared/blocks/catalogo'
import CampoDeEsquema from './CampoDeEsquema.vue'

// El formulario se arma desde el catalogo REAL del api: si un esquema cambia de
// forma que el editor no entiende, estas pruebas lo ven.
function montar(tipo: 'hero' | 'features' | 'texto', valor: unknown, errores: Record<string, string> = {}) {
  return mount(CampoDeEsquema, {
    props: {
      esquema: esquemas[tipo]!,
      etiqueta: tipo,
      ruta: 'blocks[0].props',
      errores,
      requerido: true,
      raiz: true,
      modelValue: valor,
    },
  })
}

const ultimo = (w: ReturnType<typeof montar>) => w.emitted('update:modelValue')!.at(-1)![0]
const campo = (w: ReturnType<typeof montar>, ruta: string) => w.find(`[data-campo="${ruta}"]`)

describe('CampoDeEsquema', () => {
  it('pinta un campo por propiedad, con la etiqueta del esquema', () => {
    const w = montar('hero', { title: 'Hola' })
    expect(w.text()).toContain('Titulo')
    expect(w.text()).toContain('Bajada')
    expect((campo(w, 'blocks[0].props.title').element as HTMLInputElement).value).toBe('Hola')
  })

  // maxLength 300 o mas es texto largo: la bajada no cabe en una linea.
  it('elige area de texto para lo largo y entrada para lo corto', () => {
    const w = montar('hero', { title: 'Hola' })
    expect(campo(w, 'blocks[0].props.title').element.tagName).toBe('INPUT')
    expect(campo(w, 'blocks[0].props.subtitle').element.tagName).toBe('TEXTAREA')
  })

  // El enlace del hero admite 500 caracteres, pero es una linea: lo delata el pattern.
  it('un texto largo con patron sigue siendo una linea', () => {
    const w = montar('hero', { title: 'Hola', cta: { label: 'Ver', href: '/x' } })
    expect(campo(w, 'blocks[0].props.cta.href').element.tagName).toBe('INPUT')
  })

  it('escribir actualiza solo esa propiedad', async () => {
    const w = montar('hero', { title: 'Hola', subtitle: 'Bajada' })
    await campo(w, 'blocks[0].props.title').setValue('Adios')
    expect(ultimo(w)).toEqual({ title: 'Adios', subtitle: 'Bajada' })
  })

  // "Sin bajada" no es una bajada "": vaciar un opcional lo quita.
  // Cada accion parte de la anterior: sin nadie escuchando el update,
  // defineModel guarda el valor en local, como lo haria el editor.
  it('vaciar un opcional lo quita del objeto; vaciar un obligatorio lo deja vacio', async () => {
    const w = montar('hero', { title: 'Hola', subtitle: 'Bajada' })
    await campo(w, 'blocks[0].props.subtitle').setValue('')
    expect(ultimo(w)).toEqual({ title: 'Hola' })

    await campo(w, 'blocks[0].props.title').setValue('')
    expect(ultimo(w)).toEqual({ title: '' })
  })

  it('un objeto opcional se agrega con lo obligatorio y se puede quitar', async () => {
    const w = montar('hero', { title: 'Hola' })
    expect(campo(w, 'blocks[0].props.cta.label').exists()).toBe(false)

    await w.findAll('button').find((b) => b.text().includes('Agregar llamado'))!.trigger('click')
    expect(ultimo(w)).toEqual({ title: 'Hola', cta: { label: '', href: '' } })

    await w.setProps({ modelValue: { title: 'Hola', cta: { label: 'Ver', href: '/x' } } })
    expect(campo(w, 'blocks[0].props.cta.label').exists()).toBe(true)
    await w.findAll('button').find((b) => b.text().includes('Quitar llamado'))!.trigger('click')
    expect(ultimo(w)).toEqual({ title: 'Hola' })
  })

  it('una lista agrega, reordena y quita, respetando minItems', async () => {
    const w = montar('features', { items: [{ title: 'Uno' }, { title: 'Dos' }] })
    const boton = (texto: string) => w.findAll('button').find((b) => b.attributes('aria-label') === texto || b.text() === texto)!

    await boton('Bajar Punto 1').trigger('click')
    expect(ultimo(w)).toEqual({ items: [{ title: 'Dos' }, { title: 'Uno' }] })

    await boton('Quitar Punto 2').trigger('click')
    expect(ultimo(w)).toEqual({ items: [{ title: 'Dos' }] })

    await w.findAll('button').find((b) => b.text().startsWith('Agregar'))!.trigger('click')
    expect(ultimo(w)).toEqual({ items: [{ title: 'Dos' }, { title: '' }] })

    // Con un solo punto no se puede quitar: el esquema pide al menos uno.
    await w.setProps({ modelValue: { items: [{ title: 'Solo' }] } })
    expect(boton('Quitar Punto 1').attributes('disabled')).toBeDefined()
  })

  // El 400 del api nombra el campo con la misma ruta que el formulario.
  it('el error del api llega al campo que lo tiene, dentro de listas tambien', () => {
    const w = montar(
      'features',
      { items: [{ title: 'Uno' }, { title: '' }] },
      { 'blocks[0].props.items[1].title': 'Este campo es obligatorio' },
    )
    const mal = campo(w, 'blocks[0].props.items[1].title')
    expect(mal.attributes('aria-invalid')).toBe('true')
    expect(campo(w, 'blocks[0].props.items[0].title').attributes('aria-invalid')).toBeUndefined()
    expect(w.text()).toContain('Este campo es obligatorio')
  })

  it('lo que no sabe pintar lo muestra y no lo toca', () => {
    const w = mount(CampoDeEsquema, {
      props: { esquema: { type: 'number' }, etiqueta: 'Precio', ruta: 'x', errores: {}, modelValue: 42 },
    })
    expect(w.text()).toContain('no se puede editar desde aqui')
    expect(w.find('pre').text()).toBe('42')
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })

  it('deshabilitado no deja escribir ni agregar', () => {
    const w = mount(CampoDeEsquema, {
      props: { esquema: esquemas.features!, etiqueta: 'f', ruta: 'p', errores: {}, requerido: true, raiz: true, deshabilitado: true, modelValue: { items: [{ title: 'Uno' }] } },
    })
    expect(w.find('[data-campo="p.items[0].title"]').attributes('disabled')).toBeDefined()
    expect(w.findAll('button').some((b) => b.text().startsWith('Agregar'))).toBe(false)
  })

  // format: color (platform/esquema.FormatoColor). El esquema va a mano: el real
  // es de settings, y shared/ no mira modulos. EditorDeSetting prueba el real.
  describe('un color', () => {
    const tema = { type: 'object', required: ['accent'], properties: { accent: { title: 'Acento', type: 'string', format: 'color' } } }

    function montarColor(valor: unknown, deshabilitado = false) {
      return mount(CampoDeEsquema, {
        props: { esquema: tema, etiqueta: 'Tema', ruta: 'value', errores: {}, requerido: true, raiz: true, deshabilitado, modelValue: valor },
      })
    }
    const paleta = (w: ReturnType<typeof montarColor>) => w.find<HTMLInputElement>('input[type="color"]')

    it('pinta la muestra con el color guardado, junto al hex', () => {
      const w = montarColor({ accent: '#2f6df6' })
      expect(paleta(w).element.value).toBe('#2f6df6')
      expect((campo(w, 'value.accent').element as HTMLInputElement).value).toBe('#2f6df6')
      expect(paleta(w).attributes('aria-label')).toContain('Acento')
    })

    it('elegir en la paleta cambia el valor', async () => {
      const w = montarColor({ accent: '#2f6df6' })
      await paleta(w).setValue('#e0531f')
      expect(ultimo(w)).toEqual({ accent: '#e0531f' })
    })

    it('escribir el hex mueve la muestra', async () => {
      const w = montarColor({ accent: '#2f6df6' })
      await campo(w, 'value.accent').setValue('#00aa55')
      expect(ultimo(w)).toEqual({ accent: '#00aa55' })
      expect(paleta(w).element.value).toBe('#00aa55')
    })

    // A medio escribir no hay color: la muestra se queda en el ultimo valido en
    // vez de ponerse negra, que pareceria un color elegido.
    it('un hex a medio escribir deja la muestra en el ultimo color valido', async () => {
      const w = montarColor({ accent: '#2f6df6' })
      await campo(w, 'value.accent').setValue('#00a')
      expect(ultimo(w)).toEqual({ accent: '#00a' })
      expect(paleta(w).element.value).toBe('#2f6df6')
    })

    it('deshabilitado no deja elegir', () => {
      expect(paleta(montarColor({ accent: '#2f6df6' }, true)).attributes('disabled')).toBeDefined()
    })
  })
})

