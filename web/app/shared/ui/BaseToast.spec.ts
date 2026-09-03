// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import BaseToast from './BaseToast.vue'

const montados: Array<{ unmount: () => void }> = []

afterEach(() => {
  for (const m of montados.splice(0)) m.unmount()
  document.body.innerHTML = ''
})

function montar(props: Record<string, unknown> = {}) {
  const w = mount(BaseToast, {
    props: { title: 'Pagina publicada', open: true, ...props },
    attachTo: document.body,
  })
  montados.push(w)
  return w
}

async function asentar() {
  await nextTick()
  await new Promise((listo) => setTimeout(listo, 0))
}

// El anunciador (la region aria-live que lee el aviso en voz alta) no existe al
// montar: Reka lo pinta despues de dos cuadros de animacion, a proposito, para
// que el lector de pantalla no lo lea antes de que el aviso este en el arbol.
// Esperar un tick y ya deja una prueba que pasa o falla segun lo cargada que
// este la maquina, que es peor que no tenerla.
async function esperarAnunciador(): Promise<Element> {
  for (let intento = 0; intento < 60; intento++) {
    const region = document.querySelector('[aria-live]')
    if (region) return region
    await new Promise((listo) => requestAnimationFrame(() => listo(null)))
  }
  throw new Error('el anunciador aria-live nunca aparecio')
}

describe('BaseToast', () => {
  const aviso = () => document.querySelector('.aviso')

  it('cerrado no pinta el aviso', async () => {
    montar({ open: false })
    await asentar()
    expect(aviso()).toBeNull()
  })

  it('abierto pinta titulo y descripcion', async () => {
    montar({ description: 'Ya se ve en la landing.' })
    await asentar()

    expect(aviso()?.textContent).toContain('Pagina publicada')
    expect(aviso()?.textContent).toContain('Ya se ve en la landing.')
  })

  // Un aviso que aparece en un div cualquiera no existe para quien no ve la
  // pantalla. La region con aria-live es lo que hace que se lea al aparecer.
  it('vive en una region que se anuncia sola', async () => {
    montar()
    await asentar()

    expect(document.querySelector('[role="region"]')).not.toBeNull()
    expect(await esperarAnunciador()).not.toBeNull()
  })

  // Lo que se lee en voz alta tiene que estar en el idioma de la aplicacion.
  // Reka trae "Notification" y "Notifications ({hotkey})" por omision, y como
  // no se ven en pantalla se quedan en ingles para siempre.
  it('lo que se anuncia esta en espanol', async () => {
    montar()
    await asentar()

    const region = document.querySelector('[role="region"]')!
    expect(region.getAttribute('aria-label')).toContain('Avisos')

    const anunciador = await esperarAnunciador()
    expect(anunciador.textContent).toContain('Aviso')
    expect(anunciador.textContent).not.toContain('Notification')
  })

  it('el boton de cerrar lo cierra', async () => {
    const aviso = montar()
    await asentar()

    const boton = document.querySelector<HTMLButtonElement>('[aria-label="Cerrar aviso"]')
    expect(boton).not.toBeNull()

    boton!.click()
    await asentar()
    expect(aviso.emitted('update:open')?.at(-1)).toEqual([false])
  })

  it('la variante cambia la clase, no el marcado', async () => {
    montar({ variant: 'danger' })
    await asentar()
    expect([...aviso()!.classList]).toContain('v-danger')
  })
})
