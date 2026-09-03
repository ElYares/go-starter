// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'

// El dialogo va por un Portal: su contenido NO cuelga del componente, cuelga de
// document.body. Por eso todo se busca en el documento y no en el wrapper, y por
// eso hace falta attachTo: sin un arbol conectado no hay foco que valga.
// Cada montaje se apunta aqui y se desmonta al terminar el caso. Sin esto, el
// contenido portado se queda en document.body de un caso al siguiente, y las
// busquedas por documento encuentran el dialogo de la prueba ANTERIOR: salen
// fallos y aciertos que no tienen que ver con lo que se esta afirmando.
const montados: Array<{ unmount: () => void }> = []

afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(opciones: Parameters<typeof mount>[1]) {
  const w = mount(BaseDialog, { attachTo: document.body, ...opciones } as never)
  montados.push(w)
  return w
}

function abrir(props: Record<string, unknown> = {}) {
  return montar({
    props: { title: 'Borrar pagina', open: true, ...props },
    slots: { default: '<input data-t="primero"><input data-t="segundo">' },
  })
}

async function asentar() {
  await nextTick()
  await new Promise((listo) => setTimeout(listo, 0))
}

describe('BaseDialog', () => {
  it('cerrado no deja nada en el documento', () => {
    montar({ props: { title: 'Borrar pagina' } })
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })

  it('abierto se anuncia con su titulo', async () => {
    abrir({ description: 'Esta accion no se puede deshacer.' })
    await asentar()

    const dialogo = document.querySelector('[role="dialog"]')
    expect(dialogo).not.toBeNull()

    // aria-labelledby y no un aria-label a mano: asi el titulo visible y el que
    // se anuncia son el mismo texto y no pueden separarse al editar uno.
    const idTitulo = dialogo!.getAttribute('aria-labelledby')
    expect(document.getElementById(idTitulo!)?.textContent).toContain('Borrar pagina')

    const idDesc = dialogo!.getAttribute('aria-describedby')
    expect(document.getElementById(idDesc!)?.textContent).toContain('no se puede deshacer')
  })

  // Un aria-describedby apuntando a un id que no existe es peor que no tenerlo:
  // el lector de pantalla no lee nada donde el marcado promete una explicacion.
  // Reka lo cablea siempre, asi que sin descripcion hay que quitarlo a mano.
  it('sin descripcion no deja un aria-describedby colgando', async () => {
    abrir()
    await asentar()

    const dialogo = document.querySelector('[role="dialog"]')!
    const id = dialogo.getAttribute('aria-describedby')
    expect(id === null || document.getElementById(id) !== null).toBe(true)
  })

  // El criterio de la HU. Sin foco atrapado, tabular desde el dialogo se va a la
  // pagina de detras —que sigue ahi, solo tapada por el velo— y quien navega con
  // teclado termina rellenando un formulario que no ve.
  it('atrapa el foco dentro mientras esta abierto', async () => {
    // Un boton de la pagina de DETRAS. Sin el, la prueba solo comprobaria que
    // el dialogo se lleva el foco al abrirse —que es otra cosa, y que sigue
    // pasando aunque se desactive la trampa—. La trampa es lo que impide SALIR.
    const fuera = document.createElement('button')
    fuera.textContent = 'un boton de la pagina de detras'
    document.body.append(fuera)

    abrir()
    await asentar()

    const dialogo = document.querySelector('[role="dialog"]')!
    expect(dialogo.contains(document.activeElement)).toBe(true)

    fuera.focus()
    await asentar()
    expect(
      dialogo.contains(document.activeElement),
      'el foco se escapo del dialogo: tabular desde dentro llega a la pagina tapada',
    ).toBe(true)

    // Y el resto de la pagina queda marcado como inerte para la accesibilidad,
    // que es la otra mitad: sin esto el lector de pantalla sigue paseando por
    // el contenido de detras aunque el foco no llegue.
    expect(document.body.getAttribute('aria-hidden')).toBe(null)
    const tapados = [...document.body.children].filter(
      (n) => n.getAttribute('aria-hidden') === 'true',
    )
    expect(tapados.length).toBeGreaterThan(0)
  })

  it('Escape lo cierra', async () => {
    const dialogo = abrir()
    await asentar()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await asentar()

    expect(dialogo.emitted('update:open')?.at(-1)).toEqual([false])
  })

  it('el boton de cerrar por omision tambien lo cierra', async () => {
    const dialogo = abrir()
    await asentar()

    const boton = [...document.querySelectorAll('button')].find(
      (b) => b.textContent?.trim() === 'Cerrar',
    )
    expect(boton, 'el pie trae un boton de cerrar cuando nadie pasa acciones').toBeTruthy()

    boton!.click()
    await asentar()
    expect(dialogo.emitted('update:open')?.at(-1)).toEqual([false])
  })

  it('las acciones propias reemplazan el pie, no se suman', async () => {
    montar({
      props: { title: 'Borrar pagina', open: true },
      slots: { acciones: '<button>Borrar</button>' },
      global: { components: { BaseButton } },
    })
    await asentar()

    const textos = [...document.querySelectorAll('[role="dialog"] button')].map((b) =>
      b.textContent?.trim(),
    )
    expect(textos).toContain('Borrar')
    expect(textos).not.toContain('Cerrar')
  })
})
