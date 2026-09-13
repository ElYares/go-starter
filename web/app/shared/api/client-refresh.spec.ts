import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './errors'
import { CABECERA_CSRF, candadoDelNavegador, crearCliente, type Candado } from './client'

/**
 * Un servidor de sesion falso, con estado: el `at` caduca, el refresh rota
 * `XSRF-TOKEN` y vuelve a validar el `at`, y cada pestana es un cliente que
 * comparte el MISMO cookie jar, como en un navegador.
 */
function servidorDeSesion(
  opciones: {
    refreshResponde?: number
    // Un 401 del refresh que NO borra las cookies: un proxy que se come el
    // Set-Cookie, o un servidor de otro fork que no las limpia.
    refreshNoBorraCookies?: boolean
    recursoResponde?: number
    conPista?: boolean
    conCsrf?: boolean
  } = {},
) {
  const jar = new Map<string, string>()
  if (opciones.conPista ?? true) jar.set('has_session', '1')
  if (opciones.conCsrf ?? true) jar.set('XSRF-TOKEN', 'x-0')

  const estado = { atValido: false, refreshes: 0, llamadas: [] as string[], csrfEnviados: [] as Array<string | undefined> }

  const cookies = () => [...jar].map(([k, v]) => `${k}=${v}`).join('; ')

  const fetchFalso = (async (url: string, init: RequestInit) => {
    const ruta = url.replace('/api/v1', '')
    const cabeceras = init.headers as Record<string, string>
    estado.llamadas.push(`${init.method} ${ruta}`)
    estado.csrfEnviados.push(cabeceras[CABECERA_CSRF])

    // Como la cadena global de Go: todo GET sin cookie CSRF la siembra.
    if (init.method === 'GET' && !jar.has('XSRF-TOKEN')) jar.set('XSRF-TOKEN', 'x-sembrado')

    if (ruta === '/auth/refresh') {
      estado.refreshes++
      // Una pausa real: es lo que deja que otras peticiones lleguen mientras
      // el refresh esta en vuelo.
      await new Promise((r) => setTimeout(r, 5))
      const status = opciones.refreshResponde ?? 204
      if (status !== 204) {
        if (status === 401 && !opciones.refreshNoBorraCookies) {
          jar.delete('has_session')
          jar.delete('XSRF-TOKEN')
        }
        return problema(status)
      }
      jar.set('XSRF-TOKEN', `x-${estado.refreshes}`)
      estado.atValido = true
      return new Response(null, { status: 204 })
    }
    if (ruta === '/healthz') return json({ status: 'ok' })
    if (ruta === '/auth/login') return problema(401)
    if (opciones.recursoResponde) return problema(opciones.recursoResponde)
    if (!estado.atValido) return problema(401)
    return json({ ok: true })
  }) as typeof fetch

  const pestana = (candado?: Candado) => crearCliente({ base: '/api/v1', fetch: fetchFalso, cookies, candado })
  return { jar, estado, pestana }
}

const json = (cuerpo: unknown) =>
  new Response(JSON.stringify(cuerpo), { status: 200, headers: { 'Content-Type': 'application/json' } })
const problema = (status: number) =>
  new Response(JSON.stringify({ status, code: status === 401 ? 'UNAUTHENTICATED' : 'X', traceId: 't' }), {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  })

/** Un candado de verdad entre "pestanas": serializa, como navigator.locks. */
function candadoCompartido(): Candado {
  let cola = Promise.resolve()
  return (_nombre, trabajo) => {
    const turno = cola.then(trabajo)
    cola = turno.catch(() => {})
    return turno
  }
}

/** Falla si la promesa no termina: el sintoma de CU-002 E3 es un timeout, no un error. */
function sinColgarse<T>(p: Promise<T>, ms = 500): Promise<T> {
  return Promise.race([p, new Promise<T>((_, no) => setTimeout(() => no(new Error('la peticion no volvio')), ms))])
}

describe('el refresh del cliente', () => {
  // CU-002 criterio 1.
  it('con el at caducado renueva y reintenta, y quien llama recibe los datos', async () => {
    const { estado, pestana } = servidorDeSesion()

    await expect(pestana().get('/auth/me')).resolves.toEqual({ ok: true })

    expect(estado.llamadas).toEqual(['GET /auth/me', 'POST /auth/refresh', 'GET /auth/me'])
  })

  // CU-002 criterio 3.
  it('cinco peticiones con 401 a la vez sacan UN solo refresh', async () => {
    const { estado, pestana } = servidorDeSesion()
    const api = pestana()

    const todas = await Promise.all([1, 2, 3, 4, 5].map((n) => api.get(`/recurso/${n}`)))

    expect(todas).toHaveLength(5)
    expect(estado.refreshes).toBe(1)
  })

  // CU-002 criterio 4 y E3: el 401 del refresh no entra a su propio
  // interceptor. El sintoma de equivocarse no es un error: es una promesa que
  // no vuelve.
  it('un 401 en el refresh termina la peticion, sin reintentar ni colgarse', async () => {
    const { estado, pestana } = servidorDeSesion({ refreshResponde: 401 })

    const e = await sinColgarse(pestana().get('/auth/me').catch((x: unknown) => x))

    expect(e).toBeInstanceOf(ApiError)
    expect((e as ApiError).status).toBe(401)
    expect(estado.refreshes).toBe(1)
  })

  // CU-002 criterio 5: el refresh rota el CSRF, y lo que sale despues —el
  // propio reintento y la siguiente mutacion— lleva el valor nuevo.
  it('tras renovar, las mutaciones mandan el token CSRF rotado', async () => {
    const { estado, pestana } = servidorDeSesion()
    const api = pestana()

    await api.post('/recurso', { a: 1 })
    await api.post('/otro', { b: 2 })

    expect(estado.llamadas).toEqual(['POST /recurso', 'POST /auth/refresh', 'POST /recurso', 'POST /otro'])
    expect(estado.csrfEnviados).toEqual(['x-0', 'x-0', 'x-1', 'x-1'])
  })

  // Sin pista no hubo sesion: el 401 es la respuesta correcta, y un refresh
  // seria otro 401 en cada carga anonima.
  it('sin has_session no intenta renovar', async () => {
    const { estado, pestana } = servidorDeSesion({ conPista: false })

    await expect(pestana().get('/auth/me')).rejects.toBeInstanceOf(ApiError)

    expect(estado.refreshes).toBe(0)
  })

  // El 401 del login son credenciales malas, no una sesion caducada.
  it('el 401 del login no dispara un refresh', async () => {
    const { estado, pestana } = servidorDeSesion()

    await expect(pestana().post('/auth/login', {})).rejects.toBeInstanceOf(ApiError)

    expect(estado.refreshes).toBe(0)
  })

  it('un segundo 401 despues de renovar no vuelve a renovar', async () => {
    const { estado, pestana } = servidorDeSesion()
    const api = pestana()
    // El refresh "funciona" pero el recurso sigue diciendo 401.
    const original = estado
    Object.defineProperty(original, 'atValido', { get: () => false, set: () => {} })

    await expect(sinColgarse(api.get('/auth/me'))).rejects.toBeInstanceOf(ApiError)

    expect(estado.refreshes).toBe(1)
  })

  // Una caida del refresh no es "no hay sesion": sale como caida, para que el
  // guard muestre el error en vez de mandar al login.
  it('si el refresh cae, sale la caida y no un 401', async () => {
    const { pestana } = servidorDeSesion({ refreshResponde: 502 })

    const e = await pestana().get('/auth/me').catch((x: unknown) => x)

    expect((e as ApiError).unavailable).toBe(true)
  })

  // Tras reiniciar el navegador, la cookie CSRF —de sesion— ya no esta y
  // `has_session` si. El GET siembra una nueva, y compararla con "nada"
  // pareceria que otra pestana ya renovo: se saltaria el refresh y un `rt`
  // valido acabaria en el login.
  it('con la cookie CSRF perdida al reiniciar el navegador, igual renueva', async () => {
    const { estado, pestana } = servidorDeSesion({ conCsrf: false })

    await expect(pestana().get('/auth/me')).resolves.toEqual({ ok: true })

    expect(estado.refreshes).toBe(1)
  })
})

describe('el refresh, casos de borde', () => {
  // El refresh interno no pasa por la etapa de renovacion, pero alguien puede
  // pedir /auth/refresh a mano. Su 401 no puede renovarse a si mismo.
  //
  // Con las cookies intactas a proposito: el servidor de verdad borra
  // `has_session` en ese 401, y sin pista el cliente tampoco renovaria, asi que
  // la exclusion no se estaria probando. Paso: la mutacion que la quitaba
  // sobrevivio.
  it('pedir /auth/refresh a mano con 401 no dispara otro refresh', async () => {
    const { estado, pestana } = servidorDeSesion({ refreshResponde: 401, refreshNoBorraCookies: true })

    await expect(sinColgarse(pestana().post('/auth/refresh'))).rejects.toBeInstanceOf(ApiError)

    expect(estado.refreshes).toBe(1)
  })

  // Solo el 401 dice "sesion caducada". Un 403 es un permiso que falta, y
  // renovar no lo arregla: solo gasta un `rt` y rota las cookies de todas las
  // pestanas por nada.
  it.each([403, 404, 500])('un %s no dispara un refresh', async (status) => {
    const { estado, pestana } = servidorDeSesion({ recursoResponde: status })

    await expect(pestana().get('/recurso')).rejects.toBeInstanceOf(ApiError)

    expect(estado.refreshes).toBe(0)
  })

  // La promesa compartida se suelta al terminar. Si se quedara, la siguiente
  // caducidad —quince minutos despues— reusaria un refresh ya resuelto, no
  // renovaria nada y mandaria al login a alguien con sesion.
  it('una segunda caducidad, mas tarde, vuelve a renovar', async () => {
    const { estado, pestana } = servidorDeSesion()
    const api = pestana()

    await api.get('/auth/me')
    estado.atValido = false
    await expect(api.get('/auth/me')).resolves.toEqual({ ok: true })

    expect(estado.refreshes).toBe(2)
  })
})

describe('el refresh entre pestanas', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  // CU-002 A1, resuelto en el cliente: dos pestanas del mismo navegador con el
  // mismo `rt`. Sin candado, las dos renuevan, la segunda presenta un `rt` ya
  // usado y el servidor revoca TODAS las sesiones.
  it('dos pestanas con 401 a la vez sacan UN solo refresh', async () => {
    const { estado, pestana } = servidorDeSesion()
    const candado = candadoCompartido()
    const unaPestana = pestana(candado)
    const otraPestana = pestana(candado)

    const [a, b] = await Promise.all([unaPestana.get('/auth/me'), otraPestana.get('/auth/me')])

    expect(a).toEqual({ ok: true })
    expect(b).toEqual({ ok: true })
    expect(estado.refreshes).toBe(1)
  })

  // Y sin candado, lo que pasaria: la prueba de que el candado es lo que las
  // ordena y no una casualidad del orden de las promesas.
  it('sin candado, las dos pestanas renuevan', async () => {
    const { estado, pestana } = servidorDeSesion()

    await Promise.all([pestana().get('/auth/me'), pestana().get('/auth/me')])

    expect(estado.refreshes).toBe(2)
  })

  // Si candadoDelNavegador ignorara navigator.locks, el candado de la app no
  // coordinaria nada y las pruebas de arriba, que usan uno falso, seguirian
  // pasando.
  it('con navigator.locks, el candado lo usa con el nombre del refresh', async () => {
    const pedidos: string[] = []
    vi.stubGlobal('navigator', {
      locks: { request: async (nombre: string, trabajo: () => Promise<void>) => (pedidos.push(nombre), trabajo()) },
    })
    let corrio = false

    await candadoDelNavegador()('go-starter:refresh', async () => {
      corrio = true
    })

    expect(pedidos).toEqual(['go-starter:refresh'])
    expect(corrio).toBe(true)
  })

  it('sin navigator.locks, el candado corre el trabajo directo', async () => {
    vi.stubGlobal('navigator', {})
    let corrio = false
    await candadoDelNavegador()('x', async () => {
      corrio = true
    })
    expect(corrio).toBe(true)
  })
})
