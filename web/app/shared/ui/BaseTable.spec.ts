// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseTable from './BaseTable.vue'

const COLUMNAS = [
  { key: 'titulo', label: 'Titulo' },
  { key: 'estado', label: 'Estado' },
  { key: 'vistas', label: 'Vistas', align: 'end' as const },
]

const FILAS = [
  { id: 'p1', titulo: 'Portada', estado: 'publicada', vistas: 120 },
  { id: 'p2', titulo: 'Contacto', estado: 'borrador', vistas: 0 },
]

describe('BaseTable', () => {
  it('pinta cabeceras y filas', () => {
    const tabla = mount(BaseTable, { props: { columns: COLUMNAS, rows: FILAS } })

    expect(tabla.findAll('thead th').map((c) => c.text())).toEqual([
      'Titulo',
      'Estado',
      'Vistas',
    ])
    expect(tabla.findAll('tbody tr')).toHaveLength(2)
    expect(tabla.findAll('tbody tr')[0]!.text()).toContain('Portada')
  })

  // scope="col" es lo que le dice a un lector de pantalla que esa celda titula
  // la columna. Sin el, una tabla es una reja de textos sueltos.
  it('las cabeceras dicen a que columna titulan', () => {
    const tabla = mount(BaseTable, { props: { columns: COLUMNAS, rows: FILAS } })
    for (const th of tabla.findAll('thead th')) {
      expect(th.attributes('scope')).toBe('col')
    }
  })

  it('un slot por columna deja al fork pintar la celda', () => {
    const tabla = mount(BaseTable, {
      props: { columns: COLUMNAS, rows: FILAS },
      slots: { 'celda-estado': '<span class="pinta">{{ params.valor }}</span>' },
    })

    expect(tabla.findAll('.pinta').map((s) => s.text())).toEqual(['publicada', 'borrador'])
    // Y el resto de columnas siguen con su valor por omision.
    expect(tabla.text()).toContain('Contacto')
  })

  // El vacio va DENTRO de la tabla y ocupando todas las columnas: puesto fuera,
  // la cabecera se queda flotando sobre la nada.
  it('sin filas muestra el vacio ocupando el ancho completo', () => {
    const tabla = mount(BaseTable, { props: { columns: COLUMNAS, rows: [] } })
    const celda = tabla.find('tbody td')

    expect(tabla.findAll('thead th')).toHaveLength(3)
    expect(celda.attributes('colspan')).toBe('3')
    expect(celda.text()).toBe('Sin resultados.')
  })

  it('el vacio se puede reemplazar', () => {
    const tabla = mount(BaseTable, {
      props: { columns: COLUMNAS, rows: [] },
      slots: { vacio: 'Todavia no hay paginas' },
    })
    expect(tabla.find('tbody td').text()).toBe('Todavia no hay paginas')
  })

  // Sin key estable, Vue reusa nodos por posicion: al reordenar o borrar una
  // fila, lo que quedo escrito en un input de la fila 2 aparece en la fila 1.
  it('usa la clave de fila que se le indique', () => {
    const tabla = mount(BaseTable, {
      props: {
        columns: [{ key: 'nombre', label: 'Nombre' }],
        rows: [{ slug: 'a', nombre: 'A' }],
        rowKey: 'slug',
      },
    })
    expect(tabla.findAll('tbody tr')).toHaveLength(1)
    expect(tabla.text()).toContain('A')
  })
})
