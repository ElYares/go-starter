// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import BaseDrawer from './BaseDrawer.vue'

// Como BaseDialog: el cajon va por un Portal y cuelga de document.body, asi que
// se busca en el documento, y cada montaje se desmonta al terminar el caso para
// que el cajon de una prueba no aparezca en la siguiente.
const montados: Array<{ unmount: () => void }> = []

afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(props: Record<string, unknown> = {}) {
  const w = mount(BaseDrawer, {
    attachTo: document.body,
    props: { title: 'Menu', ...props },
    slots: {
      trigger: '<button type="button" data-t="abrir">Abrir</button>',
      default: '<a href="#" data-t="dentro">Una seccion</a>',
    },
  } as never)
  montados.push(w)
  return w
}

async function asentar() {
  await nextTick()
  await new Promise((listo) => setTimeout(listo, 0))
}

const cajon = () => document.querySelector('[role="dialog"]')
// Reka deja el nodo hasta que termine la animacion de salida, que en happy-dom
// no termina nunca: cerrado es que no esta, o que dice closed.
const cerrado = () => (cajon()?.getAttribute('data-state') ?? 'closed') === 'closed'

describe('BaseDrawer', () => {
  it('cerrado no deja nada en el documento, solo el boton que lo abre', () => {
    montar()
    expect(cajon()).toBeNull()
    expect(document.querySelector('[data-t="abrir"]')).not.toBeNull()
  })

  it('el boton del slot trigger lo abre, con su contenido', async () => {
    montar()
    document.querySelector<HTMLElement>('[data-t="abrir"]')!.click()
    await asentar()

    expect(cajon()).not.toBeNull()
    expect(cajon()!.querySelector('[data-t="dentro"]')).not.toBeNull()
  })

  it('se anuncia con su titulo', async () => {
    montar({ title: 'go-starter', open: true })
    await asentar()

    const id = cajon()!.getAttribute('aria-labelledby')
    expect(document.getElementById(id!)?.textContent).toContain('go-starter')
  })

  it('no deja un aria-describedby colgando', async () => {
    montar({ open: true })
    await asentar()

    const id = cajon()!.getAttribute('aria-describedby')
    expect(id === null || document.getElementById(id) !== null).toBe(true)
  })

  it('el boton de cerrar lleva el nombre que se le da, y cierra', async () => {
    const w = montar({ open: true, closeLabel: 'Cerrar menu' })
    await asentar()

    const cerrar = cajon()!.querySelector<HTMLElement>('button[aria-label="Cerrar menu"]')
    expect(cerrar).not.toBeNull()
    cerrar!.click()
    await asentar()

    expect(cerrado()).toBe(true)
    expect(w.emitted('update:open')?.at(-1)).toEqual([false])
  })

  it('Escape lo cierra', async () => {
    const w = montar({ open: true })
    await asentar()

    document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await asentar()

    expect(w.emitted('update:open')?.at(-1)).toEqual([false])
  })
})
