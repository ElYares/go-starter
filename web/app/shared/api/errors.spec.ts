import { describe, expect, it } from 'vitest'
import { errorDesdeRespuesta, errorSinRespuesta, segundosDeRetryAfter } from './errors'

function problema(status: number, cuerpo: Record<string, unknown>, cabeceras: Record<string, string> = {}) {
  return new Response(JSON.stringify(cuerpo), {
    status,
    headers: { 'Content-Type': 'application/problem+json; charset=utf-8', ...cabeceras },
  })
}

describe('errorDesdeRespuesta', () => {
  it('lee code, traceId y Retry-After del problem+json', async () => {
    const e = await errorDesdeRespuesta(
      problema(429, { title: 'Demasiadas peticiones', status: 429, code: 'TOO_MANY_REQUESTS', traceId: 't-1' }, { 'Retry-After': '120' }),
    )

    expect(e.status).toBe(429)
    expect(e.code).toBe('TOO_MANY_REQUESTS')
    expect(e.traceId).toBe('t-1')
    expect(e.retryAfter).toBe(120)
  })

  // answered es el hecho: hubo respuesta. unavailable es la decision: no es
  // caida. Colapsarlas manda al login a quien solo tiene el servidor caido.
  it('un 401 esta contestado y NO es caida', async () => {
    const e = await errorDesdeRespuesta(problema(401, { status: 401, code: 'UNAUTHENTICATED', traceId: 't' }))

    expect(e.answered).toBe(true)
    expect(e.unavailable).toBe(false)
  })

  // Lo escribe Caddy, no Go: HTML o nada. Tiene que salir como ApiError y no
  // como un SyntaxError de JSON.
  it('un 502 sin JSON esta contestado y SI es caida', async () => {
    const e = await errorDesdeRespuesta(new Response('<html>Bad Gateway</html>', {
      status: 502,
      headers: { 'Content-Type': 'text/html' },
    }))

    expect(e.answered).toBe(true)
    expect(e.unavailable).toBe(true)
    expect(e.code).toBe('UNAVAILABLE')
  })

  it('un cuerpo que dice ser JSON y no lo es no revienta', async () => {
    const e = await errorDesdeRespuesta(new Response('{roto', {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    }))

    expect(e.status).toBe(500)
    expect(e.unavailable).toBe(false)
  })
})

describe('errorSinRespuesta', () => {
  it('un fallo de red no esta contestado y SI es caida', () => {
    const e = errorSinRespuesta(new TypeError('Failed to fetch'))

    expect(e.answered).toBe(false)
    expect(e.unavailable).toBe(true)
  })

  // La cancelo la propia pantalla: no dice nada del servidor.
  it('una cancelacion no esta contestada y NO es caida', () => {
    const e = errorSinRespuesta(new DOMException('abortada', 'AbortError'))

    expect(e.answered).toBe(false)
    expect(e.unavailable).toBe(false)
  })
})

describe('segundosDeRetryAfter', () => {
  it('lee segundos', () => {
    expect(segundosDeRetryAfter('90')).toBe(90)
  })

  // La cabecera la puede reescribir un proxy con una fecha HTTP; leida como
  // numero daria NaN minutos en pantalla.
  it('lee una fecha HTTP como los segundos que faltan', () => {
    const ahora = Date.parse('2026-09-13T10:00:00Z')
    expect(segundosDeRetryAfter('Sun, 13 Sep 2026 10:02:00 GMT', ahora)).toBe(120)
  })

  it('ignora lo que no entiende', () => {
    expect(segundosDeRetryAfter('pronto')).toBeUndefined()
    expect(segundosDeRetryAfter(null)).toBeUndefined()
  })
})
