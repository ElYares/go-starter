// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { BaseSelect } from '~/shared/ui'
import EditorDePagina from './EditorDePagina.vue'

type Pagina = Schemas['Pagina']

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

const pagina = (extra: Partial<Pagina> = {}): Pagina => ({
  id: 'p1',
  slug: 'precios',
  version: 3,
  title: 'Precios',
  seoTitle: null,
  seoDescription: null,
  blocks: [{ id: 'h', type: 'hero', props: { title: 'Hola' } }],
  note: null,
  draftVersionId: 'v3',
  draftNumber: 3,
  publishedVersionId: 'v2',
  publishedNumber: 2,
  createdAt: '2026-09-14T10:00:00Z',
  updatedAt: '2026-09-14T11:00:00Z',
  updatedBy: null,
  ...extra,
})

const versiones = (): Schemas['VersionesPage'] => ({
  content: [
    { id: 'v3', number: 3, title: 'Precios', note: null, published: false, createdAt: '2026-09-14T11:00:00Z', createdBy: null },
    { id: 'v2', number: 2, title: 'Precios', note: 'hero', published: true, createdAt: '2026-09-13T11:00:00Z', createdBy: null },
    { id: 'v1', number: 1, title: 'Precios', note: null, published: false, createdAt: '2026-09-12T11:00:00Z', createdBy: null },
  ],
  page: { number: 0, size: 100, totalElements: 3, totalPages: 1 },
})

const error = (status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ status, code: 'X', message: `HTTP ${status}`, answered: status !== 0, unavailable: status === 0, ...extra })

function montar(opciones: Partial<InstanceType<typeof EditorDePagina>['$props']> = {}) {
  const ops = {
    cargar: vi.fn(async () => pagina()),
    guardar: vi.fn(async (version: number) => pagina({ version: version + 1, draftVersionId: 'v4', draftNumber: 4 })),
    publicar: vi.fn(async (versionId: string) => pagina({ publishedVersionId: versionId, publishedNumber: 3 })),
    borrar: vi.fn(async () => {}),
    cargarVersiones: vi.fn(async () => versiones()),
    puedeEscribir: true,
    puedePublicar: true,
    ...opciones,
  }
  const w = mount(EditorDePagina, { props: ops as never, attachTo: document.body })
  montados.push(w)
  return { w, ops }
}

const boton = (w: ReturnType<typeof montar>['w'], texto: string) =>
  w.findAll('button').find((b) => b.text().trim().startsWith(texto))
const estadoDe = (w: ReturnType<typeof montar>['w']) => w.find('[data-estado]').attributes('data-estado')
const campo = (w: ReturnType<typeof montar>['w'], ruta: string) => w.find(`[data-campo="${ruta}"]`)

async function editarTitulo(w: ReturnType<typeof montar>['w'], valor = 'Hola de nuevo') {
  await campo(w, 'blocks[0].props.title').setValue(valor)
}

describe('EditorDePagina: cargar', () => {
  it('mientras carga muestra el esqueleto', () => {
    const { w } = montar({ cargar: () => new Promise(() => {}) })
    expect(estadoDe(w)).toBe('cargando')
  })

  it('con la pagina pinta sus datos, sus bloques, la vista previa y el historial', async () => {
    const { w } = montar()
    await flushPromises()

    expect(estadoDe(w)).toBe('listo')
    expect((campo(w, 'title').element as HTMLInputElement).value).toBe('Precios')
    expect((campo(w, 'blocks[0].props.title').element as HTMLInputElement).value).toBe('Hola')
    expect(w.find('.lienzo h1').text()).toBe('Hola')
    expect(w.text()).toContain('hay cambios sin publicar')
    expect(w.findAll('[data-version]')).toHaveLength(3)
  })

  it('un 404 dice que no existe y no ofrece reintentar', async () => {
    const { w } = montar({ cargar: async () => { throw error(404) } })
    await flushPromises()
    expect(w.text()).toContain('Esta pagina no existe')
    expect(boton(w, 'Reintentar')).toBeUndefined()
  })
})

describe('EditorDePagina: guardar', () => {
  it('sin cambios no deja guardar; editar lo habilita y avisa a la vista', async () => {
    const { w } = montar()
    await flushPromises()

    expect(boton(w, 'Guardar')!.attributes('disabled')).toBeDefined()
    await editarTitulo(w)
    expect(boton(w, 'Guardar')!.attributes('disabled')).toBeUndefined()
    expect(w.emitted('cambios')!.at(-1)).toEqual([true])
    // La vista previa es lo que hay en pantalla, no lo guardado.
    expect(w.find('.lienzo h1').text()).toBe('Hola de nuevo')
  })

  // CU-004 flujo principal: PUT con la version leida.
  it('guarda sobre la version leida y queda con la nueva', async () => {
    const { w, ops } = montar()
    await flushPromises()
    await editarTitulo(w)

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(ops.guardar).toHaveBeenCalledWith(3, {
      slug: 'precios',
      title: 'Precios',
      seoTitle: null,
      seoDescription: null,
      note: null,
      blocks: [{ id: 'h', type: 'hero', props: { title: 'Hola de nuevo' } }],
    })
    expect(w.text()).toContain('borrador version 4')
    expect(ops.cargarVersiones).toHaveBeenCalledTimes(2)
  })

  // CU-004 criterio 4 y E2: el 400 marca bloque y campo, y lo escrito se queda.
  it('un 400 marca el bloque y el campo, y no pierde lo escrito', async () => {
    const guardar = vi.fn(async () => {
      throw error(400, {
        traceId: 't-400',
        errors: [{ field: 'blocks[0].props.title', code: 'required', message: 'No puede estar vacio' }],
      })
    })
    const { w } = montar({ guardar })
    await flushPromises()
    await editarTitulo(w, '')

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-bloque="h"]').classes()).toContain('con-error')
    expect(campo(w, 'blocks[0].props.title').attributes('aria-invalid')).toBe('true')
    expect(w.text()).toContain('Hay 1 campo por corregir')
    expect(w.text()).toContain('t-400')
    expect((campo(w, 'blocks[0].props.title').element as HTMLInputElement).value).toBe('')
    expect(boton(w, 'Guardar')!.attributes('disabled')).toBeUndefined()
  })

  // CU-004 criterio 3 y E1: el 409 no guarda encima, y ofrece recargar.
  it('un 409 avisa con la hora de la otra version, conserva lo escrito y deja recargar', async () => {
    const cargar = vi
      .fn()
      .mockResolvedValueOnce(pagina())
      .mockResolvedValueOnce(pagina({ version: 4, updatedAt: '2026-09-14T12:34:00Z', title: 'Otra persona' }))
      .mockResolvedValueOnce(pagina({ version: 4, title: 'Otra persona' }))
    const guardar = vi.fn(async () => {
      throw error(409)
    })
    const { w } = montar({ cargar, guardar })
    await flushPromises()
    await editarTitulo(w, 'Mi cambio')

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    const aviso = w.find('[data-aviso="conflicto"]')
    expect(aviso.text()).toContain('Alguien guardo esta pagina')
    expect(aviso.text()).toContain('2026-09-14 12:34')
    expect(guardar).toHaveBeenCalledTimes(1)
    // Leer la version actual para decir la hora NO la aplica.
    expect((campo(w, 'blocks[0].props.title').element as HTMLInputElement).value).toBe('Mi cambio')
    expect((campo(w, 'title').element as HTMLInputElement).value).toBe('Precios')

    await boton(w, 'Descartar mis cambios')!.trigger('click')
    await flushPromises()
    expect((campo(w, 'title').element as HTMLInputElement).value).toBe('Otra persona')
    expect(w.find('[data-aviso="conflicto"]').exists()).toBe(false)
  })

  // El mismo 409 es tambien "ese slug ya lo usa otra pagina". Antes se anunciaba
  // como si alguien hubiera guardado en medio, y "descartar y recargar" no
  // arreglaba nada: el slug seguia ocupado.
  it('un 409 con la version intacta es el slug de otra pagina, no un conflicto de version', async () => {
    const guardar = vi.fn(async () => {
      throw error(409)
    })
    const { w } = montar({ guardar })
    await flushPromises()
    await campo(w, 'slug').setValue('inicio')

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-aviso="conflicto"]').exists()).toBe(false)
    expect(w.text()).toContain('Ya hay otra pagina con esta direccion')
    expect(campo(w, 'slug').attributes('aria-invalid')).toBe('true')
    expect((campo(w, 'slug').element as HTMLInputElement).value).toBe('inicio')
  })

  it('si no se puede releer tras un 409, avisa del conflicto sin hora', async () => {
    const cargar = vi.fn().mockResolvedValueOnce(pagina()).mockRejectedValueOnce(error(0))
    const guardar = vi.fn(async () => {
      throw error(409)
    })
    const { w } = montar({ cargar, guardar })
    await flushPromises()
    await editarTitulo(w, 'Mi cambio')

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-aviso="conflicto"]').text()).toContain('Alguien guardo esta pagina')
  })

  it('una caida al guardar lo dice y los cambios siguen', async () => {
    const { w } = montar({ guardar: async () => { throw error(0) } })
    await flushPromises()
    await editarTitulo(w, 'Sigue aqui')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-aviso="error"]').text()).toContain('El servidor no responde')
    expect((campo(w, 'blocks[0].props.title').element as HTMLInputElement).value).toBe('Sigue aqui')
  })
})

describe('EditorDePagina: bloques', () => {
  it('agrega un bloque del tipo elegido, con lo obligatorio de su esquema', async () => {
    const { w } = montar()
    await flushPromises()

    w.findComponent(BaseSelect).vm.$emit('update:modelValue', 'texto')
    await nextTick()
    await boton(w, 'Agregar bloque')!.trigger('click')

    expect(w.findAll('[data-bloque]').map((b) => b.attributes('data-bloque'))).toEqual(['h', 'texto-1'])
    expect(campo(w, 'blocks[1].props.body').exists()).toBe(true)
  })

  it('reordena y quita bloques', async () => {
    const { w } = montar({
      cargar: async () =>
        pagina({
          blocks: [
            { id: 'a', type: 'texto', props: { body: 'A' } },
            { id: 'b', type: 'texto', props: { body: 'B' } },
          ],
        }),
    })
    await flushPromises()
    const ids = () => w.findAll('[data-bloque]').map((b) => b.attributes('data-bloque'))

    await w.find('[aria-label="Bajar bloque 1"]').trigger('click')
    expect(ids()).toEqual(['b', 'a'])
    await w.find('[aria-label="Quitar bloque 2"]').trigger('click')
    expect(ids()).toEqual(['b'])
  })

  // CU-004 criterio 7 y E3.
  it('un tipo desconocido se muestra como no reconocido, se conserva al guardar, y avisa', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const desconocido = { id: 'x', type: 'carrusel', props: { fotos: ['a.jpg'] } }
    const { w, ops } = montar({ cargar: async () => pagina({ blocks: [pagina().blocks[0]!, desconocido] }) })
    await flushPromises()

    expect(w.find('[data-bloque="x"]').text()).toContain('Bloque no reconocido')
    expect(w.find('[data-aviso="desconocidos"]').exists()).toBe(true)

    await editarTitulo(w)
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()
    // Viaja tal cual: el servidor decide, y el editor nunca lo descarta solo.
    expect(vi.mocked(ops.guardar).mock.calls[0]![1].blocks[1]).toEqual(desconocido)
  })
})

describe('EditorDePagina: publicar', () => {
  it('publica el borrador guardado', async () => {
    const { w, ops } = montar()
    await flushPromises()

    await boton(w, 'Publicar version 3')!.trigger('click')
    await flushPromises()

    expect(ops.publicar).toHaveBeenCalledWith('v3')
    expect(w.text()).toContain('Publicada · version 3')
  })

  // Publicar apunta a una version guardada: con cambios en pantalla publicaria
  // algo distinto de lo que se ve.
  it('con cambios sin guardar no deja publicar', async () => {
    const { w } = montar()
    await flushPromises()
    await editarTitulo(w)
    expect(boton(w, 'Publicar version 3')!.attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('Guarda antes de publicar')
  })

  // CU-004 criterio 6 y A2: revertir es publicar una anterior desde el historial.
  it('revierte publicando una version anterior del historial', async () => {
    const { w, ops } = montar()
    await flushPromises()

    await w.find('[data-version="1"]').find('button').trigger('click')
    await flushPromises()

    expect(ops.publicar).toHaveBeenCalledWith('v1')
    expect(ops.cargarVersiones).toHaveBeenCalledTimes(2)
  })

  // CU-004 criterio 5 y A1, la mitad del cliente: sin el permiso no hay boton.
  it('sin content.page.publish no hay botones de publicar', async () => {
    const { w } = montar({ puedePublicar: false })
    await flushPromises()
    expect(boton(w, 'Publicar')).toBeUndefined()
    expect(w.find('[data-version="1"]').find('button').exists()).toBe(false)
  })

  // Y la del servidor: si se llama igual, el 403 se dice.
  it('un 403 al publicar dice que falta el permiso', async () => {
    const { w } = montar({ publicar: async () => { throw error(403, { traceId: 't-403' }) } })
    await flushPromises()
    await boton(w, 'Publicar version 3')!.trigger('click')
    await flushPromises()
    expect(w.find('[data-aviso="error"]').text()).toContain('No tienes permiso para publicar')
  })

  it('sin content.page.write es solo lectura', async () => {
    const { w } = montar({ puedeEscribir: false })
    await flushPromises()
    expect(boton(w, 'Guardar')).toBeUndefined()
    expect(boton(w, 'Borrar pagina')).toBeUndefined()
    expect(campo(w, 'title').attributes('disabled')).toBeDefined()
    expect(w.find('[data-aviso="solo-lectura"]').exists()).toBe(true)
  })
})

describe('EditorDePagina: borrar', () => {
  it('pide confirmacion y avisa al borrar', async () => {
    const { w, ops } = montar()
    await flushPromises()

    await boton(w, 'Borrar pagina')!.trigger('click')
    await flushPromises()
    expect(ops.borrar).not.toHaveBeenCalled()

    const confirmar = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Borrar para siempre'))
    confirmar!.click()
    await flushPromises()

    expect(ops.borrar).toHaveBeenCalledTimes(1)
    expect(w.emitted('borrada')).toHaveLength(1)
  })
})
