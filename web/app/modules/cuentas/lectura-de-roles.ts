// Como se lee un rol en la pantalla de roles: que alcanza y que le falta,
// agrupado por la parte del sitio a la que pertenece cada permiso.
//
// Vive fuera del componente para probarse sin montar nada, y porque "que le
// falta a un rol" es una comparacion contra los demas roles, no algo que el rol
// sepa de si mismo.
import type { Schemas } from '~/shared/api/generated'

type Permiso = Schemas['PermisoConcedido']
type Rol = Schemas['Rol']

export interface Fila {
  permiso: Permiso
  concedido: boolean
}

export interface Grupo {
  area: string
  filas: Fila[]
}

export type Alcance =
  | { tipo: 'nada' }
  | { tipo: 'todo' }
  | { tipo: 'parte'; faltan: number; total: number; areas: string[] }

export interface Lectura {
  alcance: Alcance
  grupos: Grupo[]
}

const porArea = (a: string, b: string) => a.localeCompare(b, 'es')

// catalogo son todos los permisos que concede algun rol. El superadmin los
// recibe todos en cada arranque, asi que en la practica es el catalogo entero;
// no hace falta pedirlo aparte.
export function catalogo(roles: Rol[]): Permiso[] {
  const porClave = new Map<string, Permiso>()
  for (const rol of roles) for (const p of rol.permissions) porClave.set(p.key, p)
  return [...porClave.values()].sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0))
}

// leer arma la tarjeta de un rol. Un rol parcial lista TODO el catalogo con lo
// que le falta marcado: la diferencia entre dos roles es justo lo que hay que
// ver antes de dar uno, y una lista de lo concedido no la muestra.
export function leer(rol: Rol, todos: Permiso[]): Lectura {
  if (rol.permissions.length === 0) return { alcance: { tipo: 'nada' }, grupos: [] }

  const tiene = new Set(rol.permissions.map((p) => p.key))
  const faltan = todos.filter((p) => !tiene.has(p.key))

  const grupos = new Map<string, Fila[]>()
  for (const permiso of todos) {
    const filas = grupos.get(permiso.area) ?? []
    filas.push({ permiso, concedido: tiene.has(permiso.key) })
    grupos.set(permiso.area, filas)
  }

  const alcance: Alcance =
    faltan.length === 0
      ? { tipo: 'todo' }
      : {
          tipo: 'parte',
          faltan: faltan.length,
          total: todos.length,
          areas: [...new Set(faltan.map((p) => p.area))].sort(porArea),
        }

  return {
    alcance,
    grupos: [...grupos.entries()]
      .sort(([a], [b]) => porArea(a, b))
      .map(([area, filas]) => ({ area, filas })),
  }
}

// enumerar une con comas y una "y" final: "A", "A y B", "A, B y C".
export function enumerar(partes: string[]): string {
  if (partes.length <= 1) return partes.join('')
  return `${partes.slice(0, -1).join(', ')} y ${partes.at(-1)}`
}
