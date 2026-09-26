// @vitest-environment happy-dom
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import ListaDeSettings from './ListaDeSettings.vue'

const montados: Array<{ unmount: () => void }> = []
afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(cargar: () => Promise<Schemas['SettingsPage']>) {
  const w = mount(ListaDeSettings, {
    props: { cargar },
    attachTo: document.body,
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
  montados.push(w)
  return w
}

const setting = (key: string, extra: Partial<Schemas['Setting']> = {}): Schemas['Setting'] => ({
  key,
  value: { name: 'go-starter' },
  isPublic: true,
  version: 1,
  updatedAt: '2026-09-13T10:00:00Z',
  updatedBy: null,
  ...extra,
})

const pagina = (content: Schemas['Setting'][], totalElements = content.length): Schemas['SettingsPage'] => ({
  content,
  page: { number: 0, size: 100, totalElements, totalPages: 1 },
})

const error = (status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ status, code: 'X', message: 'x', answered: status !== 0, unavailable: [0, 502].includes(status), ...extra })

const estadoDe = (w: ReturnType<typeof montar>) => w.find('[data-estado]').attributes('data-estado')

describe('ListaDeSettings', () => {
  it('mientras carga muestra el esqueleto y marca aria-busy', () => {
    const w = montar(() => new Promise(() => {}))

    expect(estadoDe(w)).toBe('cargando')
    expect(w.find('section').attributes('aria-busy')).toBe('true')
  })

  it('con datos pinta una fila por clave, con su visibilidad', async () => {
    const w = montar(async () => pagina([setting('site.brand'), setting('mail.from', { isPublic: false })]))
    await flushPromises()

    expect(estadoDe(w)).toBe('listo')
    expect(w.text()).toContain('site.brand')
    expect(w.text()).toContain('mail.from')
    expect(w.text()).toContain('Publica')
    expect(w.text()).toContain('Privada')
  })

  it('vacio dice que es y como llenarlo, con accion', async () => {
    const w = montar(async () => pagina([]))
    await flushPromises()

    expect(w.text()).toContain('Todavia no hay configuracion')
    expect(w.findAll('button').some((b) => b.text().includes('Volver a cargar'))).toBe(true)
  })

  // CU-003 criterio 6, la mitad del servidor: la URL directa llega a la
  // pantalla, y lo que niega es el 403 de la API.
  it('un 403 dice que falta el permiso, con la referencia, y no ofrece reintentar', async () => {
    const w = montar(async () => {
      throw error(403, { traceId: 't-403' })
    })
    await flushPromises()

    expect(w.text()).toContain('No tienes permiso para ver la configuracion')
    expect(w.text()).toContain('t-403')
    expect(w.findAll('button').some((b) => b.text().includes('Reintentar'))).toBe(false)
  })

  it('una caida dice que el servidor no responde y deja reintentar', async () => {
    let veces = 0
    const w = montar(async () => {
      veces++
      if (veces === 1) throw error(502)
      return pagina([setting('site.brand')])
    })
    await flushPromises()
    expect(w.text()).toContain('El servidor no responde')

    await w.findAll('button').find((b) => b.text().includes('Reintentar'))!.trigger('click')
    await flushPromises()

    expect(estadoDe(w)).toBe('listo')
    expect(veces).toBe(2)
  })

  // Al pulsar reintentar se vuelve a "cargando" en el acto. La prueba de arriba
  // espera a que todo termine y no lo veia: una mutacion que dejaba la pantalla
  // en error durante el reintento sobrevivio. Para la persona, eso es un boton
  // que no hace nada.
  it('mientras reintenta vuelve a mostrar que carga', async () => {
    let veces = 0
    const w = montar(() => {
      veces++
      if (veces === 1) return Promise.reject(error(502))
      return new Promise(() => {})
    })
    await flushPromises()
    expect(estadoDe(w)).toBe('error')

    await w.findAll('button').find((b) => b.text().includes('Reintentar'))!.trigger('click')

    expect(estadoDe(w)).toBe('cargando')
  })

  it('un 500 ensena la referencia para soporte', async () => {
    const w = montar(async () => {
      throw error(500, { traceId: 'abc123' })
    })
    await flushPromises()

    expect(w.text()).toContain('No se pudo cargar la configuracion')
    expect(w.text()).toContain('abc123')
  })

  it('si hay mas claves de las que caben en la pagina, lo dice', async () => {
    const w = montar(async () => pagina([setting('a.b')], 150))
    await flushPromises()

    expect(w.text()).toContain('Mostrando 1 de 150')
  })

  it('un valor enorme se recorta en vez de ensanchar la tabla', async () => {
    const w = montar(async () => pagina([setting('x.y', { value: { texto: 'z'.repeat(500) } })]))
    await flushPromises()

    const celda = w.findAll('code').map((c) => c.text()).find((t) => t.includes('texto'))!
    expect(celda.length).toBeLessThanOrEqual(80)
  })

  it('un error que no es de la API sube', async () => {
    const errores: unknown[] = []
    const w = mount(ListaDeSettings, {
      props: {
        cargar: async () => {
          throw new RangeError('bug')
        },
      },
      global: { config: { errorHandler: (e) => void errores.push(e) } },
    })
    montados.push(w)
    await flushPromises()

    expect(errores[0]).toBeInstanceOf(RangeError)
    expect(estadoDe(w)).not.toBe('error')
  })
})

describe('la lista enlaza cada clave a su editor', () => {
  it('con la ruta /admin/configuracion/{clave}', async () => {
    const w = montar(() =>
      Promise.resolve({
        content: [setting('site.brand'), setting('site.nav')],
        page: { number: 0, size: 100, totalElements: 2, totalPages: 1 },
      }),
    )
    await flushPromises()
    expect(w.findAllComponents(RouterLinkStub).map((l) => l.props('to'))).toEqual([
      '/admin/configuracion/site.brand',
      '/admin/configuracion/site.nav',
    ])
  })
})

// Con los esquemas REALES del api: el nombre, la descripcion y los title de los
// campos salen de settings/esquemas/, no de la clave.
describe('cada tarjeta se lee sin saber de JSON', () => {
  const tarjeta = (w: ReturnType<typeof montar>, clave: string) =>
    w.findAll('.tarjeta').find((t) => t.find('.pie code').text() === clave)!

  it('lleva el nombre y la descripcion del esquema, y la clave al pie', async () => {
    const w = montar(async () => pagina([setting('site.brand', { value: { name: 'go-starter', tagline: 'Una landing' } })]))
    await flushPromises()

    const t = tarjeta(w, 'site.brand')
    expect(t.find('h2').text()).toBe('Marca')
    expect(t.find('.descripcion').text()).toContain('El nombre del sitio')
    expect(t.find('.pie').text()).toContain('Actualizada el 13 sep 2026')
  })

  it('dice cada campo con su nombre, y lo vacio como vacio', async () => {
    const w = montar(async () => pagina([setting('site.brand', { value: { name: 'go-starter' } })]))
    await flushPromises()

    const pares = tarjeta(w, 'site.brand')
      .findAll('.valor')
      .map((v) => [v.find('dt').text(), v.find('dd').text()])
    expect(pares).toEqual([
      ['Nombre', 'go-starter'],
      ['Lema', 'Sin definir'],
      ['Logo', 'Sin imagen'],
    ])
    expect(tarjeta(w, 'site.brand').text()).not.toContain('{"name"')
  })

  it('un color se ve como color, con su hex', async () => {
    const w = montar(async () => pagina([setting('site.theme', { value: { accent: '#2f6df6' } })]))
    await flushPromises()

    const t = tarjeta(w, 'site.theme')
    expect(t.find('h2').text()).toBe('Tema')
    expect(t.find<HTMLElement>('.muestra').element.style.background).toMatch(/#2f6df6|rgb\(47, 109, 246\)/)
    expect(t.find('dd').text()).toBe('#2f6df6')
  })

  it('la navegacion nombra sus enlaces, sin un par al que le falte el nombre', async () => {
    const w = montar(async () => pagina([setting('site.nav', { value: [{ label: 'Inicio', href: '/' }] })]))
    await flushPromises()

    const t = tarjeta(w, 'site.nav')
    expect(t.find('dl').exists()).toBe(false)
    expect(t.find('.suelto').text()).toBe('Inicio')
  })

  it('una clave sin esquema se nombra con la clave y muestra su JSON', async () => {
    const w = montar(async () => pagina([setting('mail.from', { value: { a: 1 } })]))
    await flushPromises()

    const t = tarjeta(w, 'mail.from')
    expect(t.find('h2').text()).toBe('mail.from')
    expect(t.find('.suelto code').text()).toBe('{"a":1}')
  })
})

