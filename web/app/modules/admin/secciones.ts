// Las entradas de la navegacion del dashboard, cada una con el permiso que su
// pantalla necesita. Sin Nuxt: se prueba en node.

export interface Seccion {
  ruta: string
  etiqueta: string
  /**
   * El permiso de la operacion que la pantalla hace al abrir, tal como lo
   * declara el modulo del servidor. `null` es "toda persona con sesion".
   */
  permiso: string | null
}

/**
 * La lista de secciones. Un fork agrega su dominio aqui, con el permiso que
 * declara su modulo en `Permissions()`.
 */
export const SECCIONES: Seccion[] = [
  { ruta: '/admin', etiqueta: 'Inicio', permiso: null },
  { ruta: '/admin/paginas', etiqueta: 'Paginas', permiso: 'content.page.read' },
  { ruta: '/admin/configuracion', etiqueta: 'Configuracion', permiso: 'settings.read' },
  { ruta: '/admin/usuarios', etiqueta: 'Usuarios', permiso: 'identity.user.read' },
  { ruta: '/admin/roles', etiqueta: 'Roles', permiso: 'identity.role.read' },
  { ruta: '/admin/solicitudes', etiqueta: 'Solicitudes', permiso: 'identity.user.password' },
]

/**
 * Las secciones que se muestran a quien tiene estos permisos.
 *
 * **Ocultar es conveniencia, no seguridad** (CU-003): quien escriba la URL de
 * una seccion oculta llega a la pantalla, y lo que la niega es el 403 del
 * servidor a su peticion. Por eso esto no protege rutas —el guard solo exige
 * sesion— y la pantalla tiene un estado "sin permiso" propio.
 */
export function seccionesVisibles(permisos: readonly string[], secciones: readonly Seccion[] = SECCIONES): Seccion[] {
  return secciones.filter((s) => s.permiso === null || permisos.includes(s.permiso))
}
