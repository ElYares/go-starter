import { describe, expect, it } from 'vitest'
import { esquemas } from './esquemas'
import { fecha, LARGO_MAXIMO, resumir } from './resumen'

// Con los esquemas REALES del api: si uno cambia de forma que el resumen no
// entiende, estas pruebas lo ven.
describe('resumir', () => {
  it('la marca: una linea por campo, con el title del esquema', () => {
    expect(resumir(esquemas['site.brand'], { name: 'go-starter', tagline: 'Una landing' })).toEqual([
      { tipo: 'texto', etiqueta: 'Nombre', texto: 'go-starter' },
      { tipo: 'texto', etiqueta: 'Lema', texto: 'Una landing' },
      { tipo: 'vacio', etiqueta: 'Logo', texto: 'Sin imagen' },
    ])
  })

  it('un logo cargado se dice, no se muestra su uuid', () => {
    const [, , logo] = resumir(esquemas['site.brand'], { name: 'x', logo: '0b5c2c4e-8f7a-4d7e-9a53-2f1c3e1d9b10' })
    expect(logo).toEqual({ tipo: 'texto', etiqueta: 'Logo', texto: 'Imagen cargada' })
  })

  it('el tema: el color como color, no como texto', () => {
    expect(resumir(esquemas['site.theme'], { accent: '#2f6df6' })).toEqual([
      { tipo: 'color', etiqueta: 'Color de acento', color: '#2f6df6' },
    ])
  })

  // Un valor viejo que ya no cumple el esquema no se pinta como color: se ve
  // lo que hay, para que alguien lo arregle.
  it('un color invalido se muestra como texto', () => {
    expect(resumir(esquemas['site.theme'], { accent: 'azul' })).toEqual([
      { tipo: 'texto', etiqueta: 'Color de acento', texto: 'azul' },
    ])
  })

  // La navegacion es una lista entera: una sola linea, sin etiqueta, con el
  // texto de cada enlace y no su href.
  it('la navegacion nombra los enlaces por su texto', () => {
    expect(resumir(esquemas['site.nav'], [{ label: 'Inicio', href: '/' }])).toEqual([
      { tipo: 'texto', etiqueta: undefined, texto: 'Inicio' },
    ])
  })

  it('una lista larga nombra tres y cuenta el resto', () => {
    const enlaces = ['Inicio', 'Precios', 'Blog', 'Equipo', 'Contacto'].map((label) => ({ label, href: '/' }))
    expect(resumir(esquemas['site.nav'], enlaces)[0]).toMatchObject({ texto: 'Inicio, Precios, Blog y 2 mas' })
  })

  it('el pie: el texto, y una lista vacia dice que no hay', () => {
    expect(resumir(esquemas['site.footer'], { text: 'Hecho con go-starter', links: [] })).toEqual([
      { tipo: 'texto', etiqueta: 'Texto', texto: 'Hecho con go-starter' },
      { tipo: 'vacio', etiqueta: 'Enlaces', texto: 'Ninguno' },
    ])
  })

  it('un texto largo se recorta', () => {
    const [nombre] = resumir(esquemas['site.brand'], { name: 'z'.repeat(500) })
    expect(nombre).toMatchObject({ tipo: 'texto' })
    expect((nombre as { texto: string }).texto).toHaveLength(LARGO_MAXIMO)
  })

  it('sin esquema muestra el JSON, recortado', () => {
    expect(resumir(undefined, { a: 1 })).toEqual([{ tipo: 'json', texto: '{"a":1}' }])
    const [largo] = resumir(undefined, { texto: 'z'.repeat(500) })
    expect((largo as { texto: string }).texto).toHaveLength(LARGO_MAXIMO)
  })
})

describe('fecha', () => {
  it('dice el dia de la marca de tiempo, en corto', () => {
    expect(fecha('2026-09-18T23:30:00Z')).toBe('18 sep 2026')
    expect(fecha('2026-01-03T00:00:00Z')).toBe('3 ene 2026')
  })

  it('algo que no entiende lo deja como venia', () => {
    expect(fecha('ayer')).toBe('ayer')
  })
})
