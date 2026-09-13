import { describe, expect, it } from 'vitest'
import { ApiError } from './errors'
import { CABECERA_CSRF, crearCliente, leerCookie } from './client'

type Llamada = { url: string; init: RequestInit }

/**
 * Un servidor falso que sustituye a `fetch`, no al cliente: la peticion recorre
 * el codigo real de principio a fin. `responder` decide que contesta, y puede
 * cambiar las cookies como lo haria un Set-Cookie.
 */
function servidorFalso(responder: (l: Llamada, jar: { cookies: string }) => Response | Promise<Response>) {
  const jar = { cookies: '' }
  const llamadas: Llamada[] = []
  const api = crearCliente({
    base: '/api/v1',
    cookies: () => jar.cookies,
    fetch: (async (url: string, init: RequestInit) => {
      const l = { url, init }
      llamadas.push(l)
      return responder(l, jar)
    }) as typeof fetch,
  })
  return { api, jar, llamadas }
}

const cabecera = (l: Llamada, nombre: string) => (l.init.headers as Record<string, string>)[nombre]
const json = (cuerpo: unknown, status = 200) =>
  new Response(JSON.stringify(cuerpo), { status, headers: { 'Content-Type': 'application/json' } })

describe('leerCookie', () => {
  it('encuentra la cookie por nombre exacto, no por prefijo', () => {
    expect(leerCookie('XSRF-TOKEN', 'OTRO-XSRF-TOKEN=mal; XSRF-TOKEN=bien')).toBe('bien')
  })

  it('no se confunde con un valor que contiene =', () => {
    expect(leerCookie('t', 't=a=b')).toBe('a=b')
  })

  it('devuelve undefined si no esta', () => {
    expect(leerCookie('has_session', 'XSRF-TOKEN=x')).toBeUndefined()
  })
})

describe('el CSRF', () => {
  it('una mutacion lleva la cookie copiada en la cabecera', async () => {
    const { api, jar, llamadas } = servidorFalso(() => new Response(null, { status: 204 }))
    jar.cookies = 'XSRF-TOKEN=abc'

    await api.post('/auth/login', {})

    expect(cabecera(llamadas[0]!, CABECERA_CSRF)).toBe('abc')
  })

  // El login rota el token. Una copia en memoria mandaria el viejo y el
  // siguiente POST responderia 403 justo despues de entrar.
  it('se lee en cada peticion, no se guarda', async () => {
    const { api, jar, llamadas } = servidorFalso((_, j) => {
      j.cookies = 'XSRF-TOKEN=rotado'
      return new Response(null, { status: 204 })
    })
    jar.cookies = 'XSRF-TOKEN=original'

    await api.post('/auth/login', {})
    await api.post('/otra', {})

    expect(cabecera(llamadas[0]!, CABECERA_CSRF)).toBe('original')
    expect(cabecera(llamadas[1]!, CABECERA_CSRF)).toBe('rotado')
  })

  // /login lo sirve Nuxt, no Go: abrirlo no siembra la cookie. Sin esto, el
  // primer login de un visitante nuevo responde 403 siempre.
  it('sin cookie, siembra con un GET antes de la mutacion', async () => {
    const { api, llamadas } = servidorFalso((l, j) => {
      if (l.init.method === 'GET') {
        j.cookies = 'XSRF-TOKEN=sembrado'
        return json({ status: 'ok' })
      }
      return new Response(null, { status: 204 })
    })

    await api.post('/auth/login', {})

    expect(llamadas.map((l) => `${l.init.method} ${l.url}`)).toEqual([
      'GET /api/v1/healthz',
      'POST /api/v1/auth/login',
    ])
    expect(cabecera(llamadas[1]!, CABECERA_CSRF)).toBe('sembrado')
  })

  it('un GET no lleva la cabecera ni siembra nada', async () => {
    const { api, llamadas } = servidorFalso(() => json({}))

    await api.get('/auth/me')

    expect(llamadas).toHaveLength(1)
    expect(cabecera(llamadas[0]!, CABECERA_CSRF)).toBeUndefined()
  })
})

describe('las respuestas', () => {
  it('manda credentials same-origin, nunca include', async () => {
    const { api, llamadas } = servidorFalso(() => json({}))

    await api.get('/auth/me')

    expect(llamadas[0]!.init.credentials).toBe('same-origin')
  })

  it('un 204 devuelve undefined sin intentar leer JSON', async () => {
    const { api, jar } = servidorFalso(() => new Response(null, { status: 204 }))
    jar.cookies = 'XSRF-TOKEN=x'

    await expect(api.post('/auth/login', {})).resolves.toBeUndefined()
  })

  it('serializa el cuerpo como JSON', async () => {
    const { api, jar, llamadas } = servidorFalso(() => new Response(null, { status: 204 }))
    jar.cookies = 'XSRF-TOKEN=x'

    await api.post('/auth/login', { email: 'a@b.c', password: 'p' })

    expect(llamadas[0]!.init.body).toBe('{"email":"a@b.c","password":"p"}')
    expect(cabecera(llamadas[0]!, 'Content-Type')).toBe('application/json')
  })

  it('todo fallo sale como ApiError, con el code del servidor', async () => {
    const { api } = servidorFalso(() =>
      new Response(JSON.stringify({ status: 401, code: 'UNAUTHENTICATED', traceId: 't-9' }), {
        status: 401,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    )

    const e = await api.get('/auth/me').catch((x: unknown) => x)

    expect(e).toBeInstanceOf(ApiError)
    expect((e as ApiError).code).toBe('UNAUTHENTICATED')
    expect((e as ApiError).traceId).toBe('t-9')
  })

  it('un fallo de red sale como ApiError de caida, no como TypeError', async () => {
    const { api } = servidorFalso(() => {
      throw new TypeError('Failed to fetch')
    })

    const e = await api.get('/auth/me').catch((x: unknown) => x)

    expect(e).toBeInstanceOf(ApiError)
    expect((e as ApiError).unavailable).toBe(true)
  })

  // Un proxy que responde 200 con HTML en nombre de Go.
  it('un 200 que no es JSON es caida', async () => {
    const { api } = servidorFalso(() => new Response('<html></html>', { status: 200 }))

    const e = await api.get('/auth/me').catch((x: unknown) => x)

    expect((e as ApiError).unavailable).toBe(true)
  })
})
