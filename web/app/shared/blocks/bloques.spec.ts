// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import FeaturesBlock from './FeaturesBlock.vue'
import HeroBlock from './HeroBlock.vue'
import RenderDeBloques from './RenderDeBloques.vue'
import TextoBlock from './TextoBlock.vue'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('HeroBlock', () => {
  it('pinta titulo, bajada y el llamado como enlace real', () => {
    const hero = mount(HeroBlock, {
      props: { contenido: { title: 'Hola', subtitle: 'Bajada', cta: { label: 'Entrar', href: '/admin' } } },
    })

    expect(hero.find('h1').text()).toBe('Hola')
    expect(hero.find('.bajada').text()).toBe('Bajada')
    // Un <a> con href y no un boton con @click: sin JavaScript tiene que llevar
    // a algun sitio.
    const cta = hero.find('a')
    expect(cta.attributes('href')).toBe('/admin')
    expect(cta.text()).toBe('Entrar')
  })

  // Una fila escrita a mano en la base no pasa por el esquema. Antes que mandar
  // a quien visita la portada a otro dominio, el boton no se pinta.
  it('un enlace que lleva a otro sitio no se pinta', () => {
    for (const href of ['//otro.com', '/\\otro.com', 'javascript:alert(1)']) {
      const hero = mount(HeroBlock, { props: { contenido: { title: 'Hola', cta: { label: 'Ir', href } } } })
      expect(hero.find('a').exists(), href).toBe(false)
      expect(hero.find('h1').text()).toBe('Hola')
    }
  })

  it('lo opcional del esquema no deja elementos vacios', () => {
    const hero = mount(HeroBlock, { props: { contenido: { title: 'Solo titulo' } } })
    expect(hero.find('.bajada').exists()).toBe(false)
    expect(hero.find('a').exists()).toBe(false)
  })
})

describe('FeaturesBlock', () => {
  it('pinta un elemento por punto, con su texto si lo tiene', () => {
    const features = mount(FeaturesBlock, {
      props: { contenido: { title: 'Lo que trae', items: [{ title: 'Uno', text: 'Primero' }, { title: 'Dos' }] } },
    })

    const puntos = features.findAll('li')
    expect(puntos).toHaveLength(2)
    expect(puntos[0]!.find('h3').text()).toBe('Uno')
    expect(puntos[0]!.find('p').text()).toBe('Primero')
    expect(puntos[1]!.find('p').exists()).toBe(false)
  })
})

describe('TextoBlock', () => {
  it('los saltos de linea separan parrafos, y los vacios no cuentan', () => {
    const texto = mount(TextoBlock, { props: { contenido: { body: 'Uno\n\n\nDos\n  \nTres' } } })
    expect(texto.findAll('p').map((p) => p.text())).toEqual(['Uno', 'Dos', 'Tres'])
  })

  // Quien edita escribe texto, no marcado. Si el cuerpo se pintara con v-html,
  // cualquiera con content.page.write inyectaria scripts en la landing publica.
  it('lo que parece HTML se pinta como texto', () => {
    const texto = mount(TextoBlock, { props: { contenido: { body: '<script>alert(1)</script><b>negrita</b>' } } })
    expect(texto.find('script').exists()).toBe(false)
    expect(texto.find('b').exists()).toBe(false)
    expect(texto.find('p').text()).toBe('<script>alert(1)</script><b>negrita</b>')
  })
})

describe('RenderDeBloques', () => {
  it('pinta cada bloque con su componente y en el orden de la pagina', () => {
    const pagina = mount(RenderDeBloques, {
      props: {
        bloques: [
          { id: 'b1', type: 'texto', props: { body: 'Antes' } },
          { id: 'b2', type: 'hero', props: { title: 'Portada' } },
        ],
      },
    })

    const secciones = pagina.findAll('section')
    expect(secciones.map((s) => s.classes()[0])).toEqual(['texto', 'hero'])
    expect(pagina.find('h1').text()).toBe('Portada')
  })

  // CU-005 E2: un tipo que un fork borro no tumba la pagina.
  it('un tipo desconocido no se pinta, avisa, y el resto de la pagina sale igual', () => {
    const aviso = vi.spyOn(console, 'warn').mockImplementation(() => {})

    const pagina = mount(RenderDeBloques, {
      props: {
        bloques: [
          { id: 'b1', type: 'hero', props: { title: 'Arriba' } },
          { id: 'x', type: 'carrusel', props: { fotos: [] } },
          { id: 'b3', type: 'texto', props: { body: 'Abajo' } },
        ],
      },
    })

    expect(pagina.findAll('section')).toHaveLength(2)
    expect(pagina.text()).toContain('Arriba')
    expect(pagina.text()).toContain('Abajo')
    expect(aviso).toHaveBeenCalledOnce()
    expect(aviso.mock.calls[0]![0]).toContain('"carrusel"')
    expect(aviso.mock.calls[0]![0]).toContain('"x"')
  })

  it('una pagina sin bloques no revienta', () => {
    expect(mount(RenderDeBloques, { props: { bloques: [] } }).text()).toBe('')
  })
})
