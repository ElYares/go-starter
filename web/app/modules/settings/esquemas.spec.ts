import { describe, expect, it } from 'vitest'
import { esquemas } from './esquemas'

// Los esquemas del api REALES, como los ve el navegador. Si el api agrega una
// clave o cambia una forma que el formulario no entiende, esto lo ve.
describe('los esquemas de settings en el navegador', () => {
  it('traen las cuatro claves del starter', () => {
    expect(Object.keys(esquemas).sort()).toEqual(['site.brand', 'site.footer', 'site.nav', 'site.theme'])
  })

  it('llegan con los $ref resueltos: el enlace del menu tiene sus campos', () => {
    const enlace = esquemas['site.nav']!.items!
    expect(enlace.$ref).toBeUndefined()
    expect(Object.keys(enlace.properties ?? {})).toEqual(['label', 'href'])
    expect(enlace.title).toBe('Enlace')
    expect(esquemas['site.footer']!.properties!.links!.items!.properties!.href).toBeDefined()
  })

  it('el logo de la marca es un campo de imagen', () => {
    expect(esquemas['site.brand']!.properties!.logo!.format).toBe('media-id')
  })
})
