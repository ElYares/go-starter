// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import EditorDeSetting from './EditorDeSetting.vue'

// El editor se arma con los esquemas REALES del api (settings/esquemas/): si una
// clave cambia de forma que el formulario no entiende, estas pruebas lo ven.

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

const setting = (key: string, value: unknown, extra: Partial<Schemas['Setting']> = {}): Schemas['Setting'] => ({
  key,
  value,
  isPublic: true,
  version: 3,
  updatedAt: '2026-09-18T10:00:00Z',
  updatedBy: null,
  ...extra,
})

const ID_LOGO = '01a0b5e1-0e44-78ab-8df9-a128bc8c9346'

function montar(
  clave: string,
  valor: unknown,
  opciones: {
    guardar?: (version: number, valor: unknown) => Promise<Schemas['Setting']>
    cargar?: () => Promise<Schemas['Setting']>
    subirMedio?: (archivo: File) => Promise<Schemas['Medio']>
    puedeEscribir?: boolean
  } = {},
) {
  const guardar = vi.fn(
    opciones.guardar ?? ((version: number, v: unknown) => Promise.resolve(setting(clave, v, { version: version + 1 }))),
  )
  const cargar = vi.fn(opciones.cargar ?? (() => Promise.resolve(setting(clave, valor))))
  const w = mount(EditorDeSetting, {
    props: { clave, cargar, guardar, subirMedio: opciones.subirMedio, puedeEscribir: opciones.puedeEscribir ?? true },
    attachTo: document.body,
  })
  montados.push(w)
  return { w, guardar, cargar }
}

const campo = (w: ReturnType<typeof montar>['w'], ruta: string) => w.find(`[data-campo="${ruta}"]`)
const boton = (w: ReturnType<typeof montar>['w'], texto: string) => w.findAll('button').find((b) => b.text() === texto)

const fallo400 = (errors: Array<{ field: string; code: string; message: string }>) =>
  new ApiError({ status: 400, code: 'VALIDATION_FAILED', message: 'x', answered: true, unavailable: false, errors })

describe('EditorDeSetting', () => {
  it('pinta el formulario desde el esquema de la clave', async () => {
    const { w } = montar('site.brand', { name: 'go-starter' })
    await flushPromises()

    expect(w.find('h1').text()).toBe('Marca')
    expect(w.text()).toContain('Nombre')
    expect(w.text()).toContain('Lema')
    expect(w.text()).toContain('Logo')
    expect((campo(w, 'value.name').element as HTMLInputElement).value).toBe('go-starter')
  })

  it('sin cambios no deja guardar, ni con Enter', async () => {
    const { w, guardar } = montar('site.brand', { name: 'go-starter' })
    await flushPromises()

    expect(boton(w, 'Guardar')!.attributes('disabled')).toBeDefined()
    await w.find('form').trigger('submit')
    expect(guardar).not.toHaveBeenCalled()
  })

  // Criterio 1 de CU-006.
  it('guarda el nombre nuevo sobre la version que leyo', async () => {
    const { w, guardar } = montar('site.brand', { name: 'go-starter' })
    await flushPromises()

    await campo(w, 'value.name').setValue('Otra marca')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(guardar).toHaveBeenCalledWith(3, { name: 'Otra marca' })
    expect(w.text()).not.toContain('Sin guardar')
  })

  // Criterio 2: subir un PNG deja el id en el valor, y guardar lo manda.
  it('el logo subido viaja en el guardado', async () => {
    const subirMedio = vi.fn().mockResolvedValue({
      id: ID_LOGO,
      url: `/api/v1/public/media/${ID_LOGO}`,
    } as Schemas['Medio'])
    const { w, guardar } = montar('site.brand', { name: 'go-starter' }, { subirMedio })
    await flushPromises()

    const input = w.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      value: [new File(['png'], 'logo.png', { type: 'image/png' })],
      configurable: true,
    })
    await input.trigger('change')
    await flushPromises()
    expect(w.find('img').attributes('src')).toBe(`/api/v1/public/media/${ID_LOGO}`)

    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()
    expect(guardar).toHaveBeenCalledWith(3, { name: 'go-starter', logo: ID_LOGO })
  })

  // Criterio 5: agregar un enlace, subirlo al primer lugar y guardar.
  it('el orden del menu que se ve es el que se guarda', async () => {
    const { w, guardar } = montar('site.nav', [{ label: 'Inicio', href: '/' }])
    await flushPromises()

    await boton(w, 'Agregar enlace')!.trigger('click')
    await campo(w, 'value[1].label').setValue('Precios')
    await campo(w, 'value[1].href').setValue('/precios')
    await w.find('button[aria-label="Subir Enlace 2"]').trigger('click')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(guardar).toHaveBeenCalledWith(3, [
      { label: 'Precios', href: '/precios' },
      { label: 'Inicio', href: '/' },
    ])
  })

  // Criterio 6: el 400 marca ese enlace y lo escrito se queda.
  it('un 400 marca el campo que lo tiene y no toca lo escrito', async () => {
    const { w } = montar('site.nav', [{ label: 'Inicio', href: '/' }], {
      guardar: () =>
        Promise.reject(fallo400([{ field: 'value[0].href', code: 'format', message: 'No tiene el formato que acepta este campo' }])),
    })
    await flushPromises()

    await campo(w, 'value[0].href').setValue('javascript:alert(1)')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(w.find('[data-aviso="error"]').text()).toContain('Hay 1 campo por corregir')
    expect(campo(w, 'value[0].href').attributes('aria-invalid')).toBe('true')
    expect(w.text()).toContain('No tiene el formato que acepta este campo')
    expect((campo(w, 'value[0].href').element as HTMLInputElement).value).toBe('javascript:alert(1)')
  })

  // Criterio 7: la segunda pestana recibe el 409, no escribe y conserva lo escrito.
  it('un 409 no guarda encima: avisa y ofrece recargar', async () => {
    let lecturas = 0
    const { w, cargar } = montar('site.brand', { name: 'go-starter' }, {
      cargar: () => {
        lecturas++
        return Promise.resolve(
          setting('site.brand', lecturas === 1 ? { name: 'go-starter' } : { name: 'Lo de la otra pestana' }, {
            version: lecturas === 1 ? 3 : 4,
            updatedAt: '2026-09-18T11:30:00Z',
          }),
        )
      },
      guardar: () =>
        Promise.reject(new ApiError({ status: 409, code: 'CONFLICT', message: 'x', answered: true, unavailable: false })),
    })
    await flushPromises()

    await campo(w, 'value.name').setValue('Lo mio')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    const aviso = w.find('[data-aviso="conflicto"]')
    expect(aviso.text()).toContain('2026-09-18 11:30')
    expect((campo(w, 'value.name').element as HTMLInputElement).value).toBe('Lo mio')

    await aviso.find('button').trigger('click')
    await flushPromises()
    expect(cargar).toHaveBeenCalledTimes(3)
    expect((campo(w, 'value.name').element as HTMLInputElement).value).toBe('Lo de la otra pestana')
  })

  // Criterio 8: sin settings.write se ve y no se edita. El 403 del PUT lo
  // prueba el api; ocultar es conveniencia.
  it('sin permiso de escritura es solo lectura', async () => {
    const { w } = montar('site.brand', { name: 'go-starter', logo: ID_LOGO }, { puedeEscribir: false, subirMedio: vi.fn() })
    await flushPromises()

    expect(boton(w, 'Guardar')).toBeUndefined()
    expect(w.find('[data-aviso="solo-lectura"]').exists()).toBe(true)
    expect(campo(w, 'value.name').attributes('disabled')).toBeDefined()
    expect(w.find('input[type="file"]').exists()).toBe(false)
    expect(w.find('img').exists()).toBe(true)
  })

  // Flujo 2a: una clave que agrego un fork en el api y que web no conoce.
  it('una clave sin esquema en el navegador se muestra y no se edita', async () => {
    const { w } = montar('site.redes', { x: '@go' })
    await flushPromises()

    expect(w.find('[data-aviso="sin-esquema"]').text()).toContain('@go')
    expect(w.find('form').exists()).toBe(false)
    expect(boton(w, 'Guardar')).toBeUndefined()
  })

  it('avisa a la vista cuando hay cambios sin guardar', async () => {
    const { w } = montar('site.brand', { name: 'go-starter' })
    await flushPromises()

    await campo(w, 'value.name').setValue('Otra')
    expect(w.emitted('cambios')!.at(-1)).toEqual([true])
    await boton(w, 'Descartar cambios')!.trigger('click')
    expect(w.emitted('cambios')!.at(-1)).toEqual([false])
  })

  it('sin permiso de lectura dice cual falta y no ofrece reintentar', async () => {
    const { w } = montar('site.brand', null, {
      cargar: () =>
        Promise.reject(new ApiError({ status: 403, code: 'FORBIDDEN', message: 'x', answered: true, unavailable: false })),
    })
    await flushPromises()

    expect(w.find('[data-estado="error"]').text()).toContain('settings.read')
    expect(boton(w, 'Reintentar')).toBeUndefined()
  })

  it('una caida ofrece reintentar', async () => {
    const { w } = montar('site.brand', null, {
      cargar: () =>
        Promise.reject(new ApiError({ status: 502, code: 'UNAVAILABLE', message: 'x', answered: true, unavailable: true })),
    })
    await flushPromises()

    expect(w.text()).toContain('El servidor no responde')
    expect(boton(w, 'Reintentar')).toBeDefined()
  })
})

// El esquema real de site.theme dice `format: color`: el acento se elige en la
// paleta, y lo elegido es lo que se guarda.
describe('el tema se edita con la paleta', () => {
  it('muestra el color guardado y guarda el elegido', async () => {
    const { w, guardar } = montar('site.theme', { accent: '#2f6df6' })
    await flushPromises()

    const paleta = w.find<HTMLInputElement>('input[type="color"]')
    expect(paleta.exists()).toBe(true)
    expect(paleta.element.value).toBe('#2f6df6')

    await paleta.setValue('#e0531f')
    await boton(w, 'Guardar')!.trigger('click')
    await flushPromises()

    expect(guardar.mock.calls[0]?.[1]).toEqual({ accent: '#e0531f' })
  })
})

