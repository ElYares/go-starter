import { describe, expect, it } from 'vitest'
import type { Schemas } from '~/shared/api/generated'
import { catalogo, enumerar, leer } from './lectura-de-roles'

const p = (key: string, area: string, sensitive = false): Schemas['PermisoConcedido'] => ({
  key,
  description: `Descripcion de ${key}`,
  area,
  sensitive,
})

const ver = p('content.page.read', 'Paginas')
const publicar = p('content.page.publish', 'Paginas')
const subir = p('media.upload', 'Imagenes')
const asignar = p('identity.role.assign', 'Usuarios y roles', true)

const superadmin = { key: 'superadmin', name: 'Super', permissions: [publicar, ver, asignar, subir] }
const admin = { key: 'admin', name: 'Admin', permissions: [publicar, ver, subir] }
const editor = { key: 'editor', name: 'Editor', permissions: [ver] }
const viewer = { key: 'viewer', name: 'Solo lectura', permissions: [] }

const todos = catalogo([admin, superadmin, editor, viewer])

describe('catalogo', () => {
  it('junta los permisos de todos los roles sin repetir, por clave', () => {
    expect(todos.map((x) => x.key)).toEqual([
      'content.page.publish',
      'content.page.read',
      'identity.role.assign',
      'media.upload',
    ])
  })
})

describe('leer', () => {
  it('un rol sin permisos no concede nada y no lista grupos', () => {
    expect(leer(viewer, todos)).toEqual({ alcance: { tipo: 'nada' }, grupos: [] })
  })

  it('un rol con todo el catalogo lo puede todo', () => {
    const l = leer(superadmin, todos)
    expect(l.alcance).toEqual({ tipo: 'todo' })
    expect(l.grupos.flatMap((g) => g.filas).every((f) => f.concedido)).toBe(true)
  })

  it('agrupa por area en orden alfabetico y deja cada permiso en la suya', () => {
    const l = leer(superadmin, todos)
    expect(l.grupos.map((g) => g.area)).toEqual(['Imagenes', 'Paginas', 'Usuarios y roles'])
    expect(l.grupos[1]!.filas.map((f) => f.permiso.key)).toEqual(['content.page.publish', 'content.page.read'])
  })

  it('un rol parcial lista tambien lo que le falta, marcado, y dice de que areas', () => {
    const l = leer(admin, todos)
    expect(l.alcance).toEqual({ tipo: 'parte', faltan: 1, total: 4, areas: ['Usuarios y roles'] })

    const usuarios = l.grupos.find((g) => g.area === 'Usuarios y roles')!
    expect(usuarios.filas).toEqual([{ permiso: asignar, concedido: false }])
  })

  it('las areas que faltan salen sin repetir y ordenadas', () => {
    const l = leer(editor, todos)
    expect(l.alcance).toEqual({ tipo: 'parte', faltan: 3, total: 4, areas: ['Imagenes', 'Paginas', 'Usuarios y roles'] })
  })
})

describe('enumerar', () => {
  it('une como se dice en espanol', () => {
    expect(enumerar([])).toBe('')
    expect(enumerar(['A'])).toBe('A')
    expect(enumerar(['A', 'B'])).toBe('A y B')
    expect(enumerar(['A', 'B', 'C'])).toBe('A, B y C')
  })
})
