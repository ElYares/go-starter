import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/shared/api/errors'
import {
  cerrarSesion,
  decidirAcceso,
  destinoSeguro,
  esRutaProtegida,
  iniciarSesion,
  mensajeDeLogin,
  type ApiDeSesion,
  type Perfil,
} from './sesion'

const PERFIL: Perfil = {
  id: '01a069aa-0f05-7cad-b231-4f7f51e83204',
  email: 'ana@casa.com',
  displayName: 'Ana',
  roles: ['admin'],
  permissions: ['settings.read'],
}

function apiError(status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) {
  return new ApiError({
    status,
    code: 'X',
    message: 'x',
    answered: status !== 0,
    unavailable: [502, 503, 504].includes(status),
    ...extra,
  })
}

describe('iniciarSesion', () => {
  // El login responde 204 sin cuerpo: lo que se sabe de la persona sale de me.
  it('hace login y despues pide me, en ese orden', async () => {
    const orden: string[] = []
    const api: ApiDeSesion = {
      post: vi.fn(async (ruta: string) => {
        orden.push(`POST ${ruta}`)
        return undefined as never
      }),
      get: vi.fn(async (ruta: string) => {
        orden.push(`GET ${ruta}`)
        return PERFIL as never
      }),
    }

    await expect(iniciarSesion(api, { email: 'ana@casa.com', password: 'x' })).resolves.toEqual(PERFIL)
    expect(orden).toEqual(['POST /auth/login', 'GET /auth/me'])
  })

  it('si el login falla, no pide me', async () => {
    const api: ApiDeSesion = {
      post: vi.fn(async () => {
        throw apiError(401)
      }),
      get: vi.fn(),
    }

    await expect(iniciarSesion(api, { email: 'a', password: 'b' })).rejects.toBeInstanceOf(ApiError)
    expect(api.get).not.toHaveBeenCalled()
  })
})

describe('esRutaProtegida', () => {
  it.each(['/admin', '/admin/', '/admin/paginas/123'])('%s esta protegida', (ruta) => {
    expect(esRutaProtegida(ruta)).toBe(true)
  })

  // startsWith('/admin') a secas las meteria.
  it.each(['/', '/login', '/administracion', '/admins'])('%s no', (ruta) => {
    expect(esRutaProtegida(ruta)).toBe(false)
  })
})

describe('destinoSeguro', () => {
  it('conserva la ruta profunda con su query y su hash', () => {
    expect(destinoSeguro('/admin/paginas/123?tab=seo#bloques')).toBe('/admin/paginas/123?tab=seo#bloques')
  })

  it('sin next, al dashboard', () => {
    expect(destinoSeguro(undefined)).toBe('/admin')
    expect(destinoSeguro('')).toBe('/admin')
  })

  // La redireccion abierta: el enlace tiene el dominio de verdad y, despues de
  // pedir la contrasena, deja a la persona en otro sitio.
  //
  // Con ruta profunda a proposito: con `/admin` a secas, cualquier forma de
  // aceptar el enlace ajeno devuelve lo mismo que el valor por omision y la
  // prueba no distingue nada. Paso: la mutacion que quitaba el chequeo de
  // origen sobrevivio.
  it.each([
    'https://otro.sitio/admin/pagos',
    '//otro.sitio/admin/pagos',
    '/\\otro.sitio/admin/pagos',
    'javascript:alert(1)',
  ])('rechaza %s', (next) => {
    expect(destinoSeguro(next)).toBe('/admin')
  })

  it('rechaza una ruta de este origen que no es del dashboard', () => {
    expect(destinoSeguro('/login?next=/admin')).toBe('/admin')
  })

  // Vue Router entrega un array si la query se repite.
  it('rechaza lo que no es una cadena', () => {
    expect(destinoSeguro(['/admin/a', '/admin/b'])).toBe('/admin')
  })
})

describe('mensajeDeLogin', () => {
  // Uno solo: el servidor ya responde identico para correo inexistente,
  // contrasena incorrecta y cuenta deshabilitada. Distinguir aqui rehace el
  // oraculo.
  it('un 401 dice lo mismo sin importar el detail del servidor', () => {
    const a = mensajeDeLogin(apiError(401, { message: 'La peticion no trae una sesion valida' }))
    const b = mensajeDeLogin(apiError(401, { message: 'Otra cosa' }))

    expect(a).toBe('Correo o contrasena incorrectos.')
    expect(b).toBe(a)
  })

  it('un 429 dice cuantos minutos, redondeando hacia arriba', () => {
    expect(mensajeDeLogin(apiError(429, { retryAfter: 61 }))).toContain('2 minuto(s)')
  })

  // Redondear hacia abajo daria "0 minutos", que invita a reintentar ya.
  it('un 429 de pocos segundos dice 1 minuto, no 0', () => {
    expect(mensajeDeLogin(apiError(429, { retryAfter: 5 }))).toContain('1 minuto(s)')
  })

  it('un 429 sin Retry-After no inventa un numero', () => {
    expect(mensajeDeLogin(apiError(429))).toContain('unos minutos')
  })

  it('una caida no dice que las credenciales esten mal', () => {
    expect(mensajeDeLogin(apiError(502))).toContain('servidor no responde')
    expect(mensajeDeLogin(apiError(0, { unavailable: true }))).toContain('servidor no responde')
  })

  it('una cancelacion no muestra nada', () => {
    expect(mensajeDeLogin(apiError(0, { answered: false, unavailable: false }))).toBeUndefined()
  })
})

describe('decidirAcceso', () => {
  it('con perfil ya cargado pasa sin pedir nada', async () => {
    const pedirPerfil = vi.fn()

    const acceso = await decidirAcceso({ perfil: PERFIL, pista: false, pedirPerfil })

    expect(acceso).toEqual({ tipo: 'pasar', perfil: PERFIL })
    expect(pedirPerfil).not.toHaveBeenCalled()
  })

  // Sin la pista, pedir me es un 401 que ya se sabia: ruido y latencia.
  it('sin pista de sesion va al login sin pedir me', async () => {
    const pedirPerfil = vi.fn()

    const acceso = await decidirAcceso({ perfil: null, pista: false, pedirPerfil })

    expect(acceso).toEqual({ tipo: 'login' })
    expect(pedirPerfil).not.toHaveBeenCalled()
  })

  it('con pista, pide me y pasa con lo que devuelve', async () => {
    const acceso = await decidirAcceso({ perfil: null, pista: true, pedirPerfil: async () => PERFIL })

    expect(acceso).toEqual({ tipo: 'pasar', perfil: PERFIL })
  })

  it('si me responde 401, al login', async () => {
    const acceso = await decidirAcceso({
      perfil: null,
      pista: true,
      pedirPerfil: async () => {
        throw apiError(401)
      },
    })

    expect(acceso).toEqual({ tipo: 'login' })
  })

  // El caso que se rompe al colapsar answered y unavailable.
  it.each([502, 0])('si me falla por caida (%s), error y NO login', async (status) => {
    const acceso = await decidirAcceso({
      perfil: null,
      pista: true,
      pedirPerfil: async () => {
        throw apiError(status, { unavailable: true })
      },
    })

    expect(acceso.tipo).toBe('error')
  })

  it('un 500 de me tampoco expulsa', async () => {
    const acceso = await decidirAcceso({
      perfil: null,
      pista: true,
      pedirPerfil: async () => {
        throw apiError(500)
      },
    })

    expect(acceso.tipo).toBe('error')
  })

  it('un error que no es de la API no se traga', async () => {
    await expect(
      decidirAcceso({
        perfil: null,
        pista: true,
        pedirPerfil: async () => {
          throw new RangeError('bug')
        },
      }),
    ).rejects.toBeInstanceOf(RangeError)
  })
})

describe('cerrarSesion', () => {
  it('llama al logout', async () => {
    const api: ApiDeSesion = { post: vi.fn(async () => undefined as never), get: vi.fn() }

    await cerrarSesion(api)

    expect(api.post).toHaveBeenCalledWith('/auth/logout')
  })

  // Quien pulso "cerrar sesion" sale igual: el servidor borra las cookies
  // aunque su base falle, y un error aqui lo dejaria dentro creyendo que salio.
  it('no lanza si el servidor falla', async () => {
    const api: ApiDeSesion = {
      post: vi.fn(async () => {
        throw apiError(502)
      }),
      get: vi.fn(),
    }

    await expect(cerrarSesion(api)).resolves.toBeUndefined()
  })
})
