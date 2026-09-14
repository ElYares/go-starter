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

  it('cada seccion con permiso nombra uno de la forma modulo.accion', () => {
    for (const s of SECCIONES.filter((x) => x.permiso !== null)) {
      expect(s.permiso).toMatch(/^[a-z]+\.[a-z]+$/)
    }
  })
})
