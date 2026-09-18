// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import CampoDeEsquema from './CampoDeEsquema.vue'
import CampoDeMedio from './CampoDeMedio.vue'
import { MAX_BYTES_MEDIO } from './esquema'

const ID_VIEJO = '01a0b5e1-0e44-78ab-8df9-a128bc8c9346'
const ID_NUEVO = '01a0b5e1-0e44-78ab-8df9-a128bc8c9999'

const medio = (id: string): Schemas['Medio'] => ({
  id,
  url: `/api/v1/public/media/${id}`,
  mime: 'image/png',
  sizeBytes: 10,
  width: 2,
  height: 2,
  sha256: 'a'.repeat(64),
  originalName: 'logo.png',
  createdAt: '2026-09-18T10:00:00Z',
  createdBy: null,
})

function montar(props: Partial<InstanceType<typeof CampoDeMedio>['$props']> = {}) {
  return mount(CampoDeMedio, { props: { etiqueta: 'Logo', ruta: 'value.logo', modelValue: ID_VIEJO, ...props } })
}

// happy-dom no deja escribir `files` de un input: se define a mano.
async function elegir(w: ReturnType<typeof montar>, archivo: File) {
  const input = w.find('input[type="file"]')
  Object.defineProperty(input.element, 'files', { value: [archivo], configurable: true })
  await input.trigger('change')
  await flushPromises()
}

const png = (bytes = 10) => new File([new Uint8Array(bytes)], 'logo.png', { type: 'image/png' })
const ultimo = (w: ReturnType<typeof montar>) => w.emitted('update:modelValue')?.at(-1)?.[0]

describe('CampoDeMedio', () => {
  it('muestra la imagen actual desde la ruta publica', () => {
    const w = montar()
    expect(w.find('img').attributes('src')).toBe(`/api/v1/public/media/${ID_VIEJO}`)
  })

  it('al elegir un archivo lo sube y el valor pasa a ser el id nuevo', async () => {
    const subir = vi.fn().mockResolvedValue(medio(ID_NUEVO))
    const w = montar({ subir })

    await elegir(w, png())

    expect(subir).toHaveBeenCalledOnce()
    expect(ultimo(w)).toBe(ID_NUEVO)
  })

  // Criterio 4 de CU-006: el error sale en el campo y el logo anterior se queda.
  it('si el servidor rechaza el archivo, dice por que y no cambia el valor', async () => {
    const subir = vi.fn().mockRejectedValue(
      new ApiError({
        status: 400,
        code: 'VALIDATION_FAILED',
        message: 'El archivo no es una imagen permitida',
        answered: true,
        unavailable: false,
        errors: [{ field: 'file', code: 'type', message: 'Solo PNG, JPEG o WebP; el archivo es image/gif' }],
      }),
    )
    const w = montar({ subir })

    await elegir(w, new File(['GIF89a'], 'logo.gif', { type: 'image/gif' }))

    expect(w.text()).toContain('Solo PNG, JPEG o WebP')
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })

  it('un archivo de mas de 5 MB no se manda', async () => {
    const subir = vi.fn()
    const w = montar({ subir })

    await elegir(w, png(MAX_BYTES_MEDIO + 1))

    expect(subir).not.toHaveBeenCalled()
    expect(w.text()).toContain('5 MB')
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })

  it('el 413 del servidor tambien se explica', async () => {
    const subir = vi.fn().mockRejectedValue(
      new ApiError({ status: 413, code: 'PAYLOAD_TOO_LARGE', message: 'x', answered: true, unavailable: false }),
    )
    const w = montar({ subir })
    await elegir(w, png())
    expect(w.text()).toContain('5 MB')
  })

  it('sin permiso de subir, se ve la imagen y no se puede cambiar', () => {
    const w = montar()
    expect(w.find('img').exists()).toBe(true)
    expect(w.find('input[type="file"]').exists()).toBe(false)
  })

  it('un logo opcional se puede quitar', async () => {
    const w = montar({ subir: vi.fn() })
    await w.findAll('button').find((b) => b.text().startsWith('Quitar'))!.trigger('click')
    expect(ultimo(w)).toBeUndefined()
    expect(w.emitted('update:modelValue')).toHaveLength(1)
  })
})

describe('CampoDeEsquema con format media-id', () => {
  it('pinta el campo de imagen y le pasa la subida', async () => {
    const subir = vi.fn().mockResolvedValue(medio(ID_NUEVO))
    const w = mount(CampoDeEsquema, {
      props: {
        esquema: {
          type: 'object',
          required: ['name'],
          properties: {
            name: { type: 'string', title: 'Nombre' },
            logo: { type: 'string', title: 'Logo', format: 'media-id' },
          },
        },
        etiqueta: 'Marca',
        ruta: 'value',
        errores: {},
        requerido: true,
        raiz: true,
        subirMedio: subir,
        modelValue: { name: 'X' },
      },
    })

    const input = w.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [png()], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(w.emitted('update:modelValue')!.at(-1)![0]).toEqual({ name: 'X', logo: ID_NUEVO })
  })
})
