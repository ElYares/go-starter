import { describe, expect, it } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { esquemas } from '~/shared/blocks/catalogo'
import {
  bloqueConError,
  borradorDe,
  cuerpoDeGuardado,
  estadoDePublicacion,
  hayCambios,
  nuevoBloque,
  sugerirSlug,
} from './editor'
import { erroresPorCampo, valorInicial } from '~/shared/formularios/esquema'

const pagina = (extra: Partial<Schemas['Pagina']> = {}): Schemas['Pagina'] => ({
  id: 'p1',
  slug: 'precios',
  version: 3,
  title: 'Precios',
  seoTitle: null,
  seoDescription: 'Desc',
  blocks: [{ id: 'h', type: 'hero', props: { title: 'Hola' } }],
  note: 'nota del guardado anterior',
  draftVersionId: 'v3',
  draftNumber: 3,
  publishedVersionId: 'v2',
  publishedNumber: 2,
  createdAt: '2026-09-14T10:00:00Z',
  updatedAt: '2026-09-14T11:00:00Z',
  updatedBy: null,
  ...extra,
})

describe('el borrador', () => {
  it('arranca con la nota vacia: es de cada guardado', () => {
    expect(borradorDe(pagina()).note).toBe('')
  })

  it('editar sus bloques no toca la pagina leida', () => {
    const p = pagina()
    const b = borradorDe(p)
    ;(b.blocks[0]!.props as { title: string }).title = 'Cambiado'
    expect((p.blocks[0]!.props as { title: string }).title).toBe('Hola')
    expect(hayCambios(p, b)).toBe(true)
  })

  // En el editor la pagina es reactiva: un proxy. structuredClone lanza con
  // un proxy, y el editor se quedaba en "cargando" para siempre.
  it('se puede sacar de una pagina envuelta en un proxy', () => {
    const p = pagina()
    const envuelta = new Proxy(p, {})
    const conBloquesEnvueltos = { ...p, blocks: new Proxy(p.blocks, {}) }
    expect(() => borradorDe(envuelta)).not.toThrow()
    expect(borradorDe(conBloquesEnvueltos).blocks).toEqual(p.blocks)
    expect(hayCambios(conBloquesEnvueltos, borradorDe(p))).toBe(false)
  })

  it('sin tocar nada no hay cambios; con solo una nota, si', () => {
    const p = pagina()
    const b = borradorDe(p)
    expect(hayCambios(p, b)).toBe(false)
    b.note = 'cambie el hero'
    expect(hayCambios(p, b)).toBe(true)
  })

  // Un seoTitle "" pisaria el title en las meta etiquetas con una cadena vacia.
  it('el cuerpo manda los opcionales vacios como null', () => {
    const b = { ...borradorDe(pagina()), seoTitle: '  ', note: '' }
    expect(cuerpoDeGuardado(b)).toEqual({
      slug: 'precios',
      title: 'Precios',
      seoTitle: null,
      seoDescription: 'Desc',
      note: null,
      blocks: [{ id: 'h', type: 'hero', props: { title: 'Hola' } }],
    })
  })
})

describe('estadoDePublicacion', () => {
  it('distingue nunca publicada, al dia y con cambios sin publicar', () => {
    expect(estadoDePublicacion(pagina({ publishedVersionId: null, publishedNumber: null }))).toBe('sin-publicar')
    expect(estadoDePublicacion(pagina({ publishedVersionId: 'v3' }))).toBe('al-dia')
    expect(estadoDePublicacion(pagina())).toBe('cambios-sin-publicar')
  })
})

describe('los bloques nuevos', () => {
  // Lo obligatorio nace para que el formulario tenga donde escribir; lo
  // opcional no, porque una bajada "" no es lo mismo que no tener bajada.
  it('nacen con lo obligatorio de su esquema y nada mas', () => {
    expect(valorInicial(esquemas.hero!)).toEqual({ title: '' })
    expect(valorInicial(esquemas.texto!)).toEqual({ body: '' })
    // minItems 1: una lista de puntos nace con un punto, con su titulo.
    expect(valorInicial(esquemas.features!)).toEqual({ items: [{ title: '' }] })
  })

  it('reciben un id que no usa otro bloque de la pagina', () => {
    const existentes = [
      { id: 'hero-1', type: 'hero', props: {} },
      { id: 'hero-2', type: 'hero', props: {} },
    ]
    expect(nuevoBloque('hero', esquemas.hero!, existentes).id).toBe('hero-3')
    expect(nuevoBloque('texto', esquemas.texto!, existentes)).toEqual({ id: 'texto-1', type: 'texto', props: { body: '' } })
  })
})

describe('los errores de un 400', () => {
  const fallo = new ApiError({
    status: 400,
    code: 'VALIDATION_FAILED',
    message: 'x',
    answered: true,
    unavailable: false,
    errors: [
      { field: 'blocks[1].props.title', code: 'required', message: 'Este campo es obligatorio' },
      { field: 'blocks[1].props.title', code: 'max', message: 'otro mensaje' },
      { field: 'blocks[10].type', code: 'unknown', message: 'No existe' },
      { field: 'slug', code: 'format', message: 'Solo minusculas' },
    ],
  })

  // Con un prefijo sin el corchete de cierre, blocks[1] se quedaria con los
  // errores de blocks[10].
  it('se reparten por bloque sin confundir blocks[1] con blocks[10]', () => {
    const errores = erroresPorCampo(fallo)
    expect(bloqueConError(errores, 1)).toBe(true)
    expect(bloqueConError(errores, 10)).toBe(true)
    expect(bloqueConError(errores, 0)).toBe(false)
    expect(bloqueConError({ 'blocks[1]': 'x' }, 1)).toBe(true)
    // Solo el bloque 10 tiene errores: el 1 no se marca.
    expect(bloqueConError({ 'blocks[10].props.title': 'x' }, 1)).toBe(false)
  })
})

describe('sugerirSlug', () => {
  it('deja un slug que el contrato acepta', () => {
    expect(sugerirSlug('Precios y Planes')).toBe('precios-y-planes')
    expect(sugerirSlug('  ¿Quiénes somos?  ')).toBe('quienes-somos')
    expect(sugerirSlug('Año 2026 — ñandú')).toBe('ano-2026-nandu')
    expect(sugerirSlug('!!!')).toBe('')
  })

  it('no pasa de 80 ni termina en guion al recortar', () => {
    const slug = sugerirSlug(`${'a'.repeat(79)} b`)
    expect(slug.length).toBeLessThanOrEqual(80)
    expect(slug.endsWith('-')).toBe(false)
  })
})
