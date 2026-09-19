import { describe, expect, it } from 'vitest'
import { SECCIONES, seccionesVisibles } from './secciones'

describe('seccionesVisibles', () => {
  it('sin permisos queda solo lo que no pide ninguno', () => {
    expect(seccionesVisibles([]).map((s) => s.ruta)).toEqual(['/admin'])
  })

  it('con settings.read aparece la configuracion', () => {
    expect(seccionesVisibles(['settings.read']).map((s) => s.ruta)).toEqual(['/admin', '/admin/configuracion'])
  })

  // El permiso de escritura no da la entrada: la pantalla abre leyendo, y sin
  // settings.read su primera peticion es un 403.
  it('settings.write sin settings.read no la muestra', () => {
    expect(seccionesVisibles(['settings.write']).map((s) => s.ruta)).toEqual(['/admin'])
  })

  // Comparacion exacta: `settings` no es `settings.read`, ni un prefijo sirve.
  it('no acepta un permiso por prefijo', () => {
    expect(seccionesVisibles(['settings', 'settings.re']).map((s) => s.ruta)).toEqual(['/admin'])
  })

  // CU-004: la entrada de paginas abre leyendo, asi que pide content.page.read.
  it('con content.page.read aparecen las paginas; con solo publish, no', () => {
    expect(seccionesVisibles(['content.page.read']).map((s) => s.ruta)).toEqual(['/admin', '/admin/paginas'])
    expect(seccionesVisibles(['content.page.publish']).map((s) => s.ruta)).toEqual(['/admin'])
  })

  // HU-018: usuarios y roles son dos entradas, cada una con el permiso de
  // lectura de su pantalla. El de escritura solo no las abre.
  it('usuarios y roles piden su permiso de lectura', () => {
    expect(seccionesVisibles(['identity.user.read']).map((s) => s.ruta)).toEqual(['/admin', '/admin/usuarios'])
    expect(seccionesVisibles(['identity.role.read']).map((s) => s.ruta)).toEqual(['/admin', '/admin/roles'])
    expect(seccionesVisibles(['identity.user.write', 'identity.role.assign']).map((s) => s.ruta)).toEqual(['/admin'])
  })

  // `modulo.accion` o `modulo.recurso.accion`, como los declara el api
  // (settings.read, content.page.read). Un permiso mal escrito aqui oculta la
  // entrada a todo el mundo sin un error.
  it('cada seccion con permiso nombra uno con la forma de los del api', () => {
    for (const s of SECCIONES.filter((x) => x.permiso !== null)) {
      expect(s.permiso).toMatch(/^[a-z]+(\.[a-z]+){1,2}$/)
    }
  })
})
