import type { Perfil } from '../sesion'

/**
 * El perfil de quien tiene la sesion, o `null`.
 *
 * `useState` y no un `ref` de modulo: en el servidor de Nuxt un `ref` suelto
 * vive lo que vive el proceso y lo comparten todas las peticiones, asi que el
 * perfil de una persona apareceria en la pagina de otra. Hoy solo lo usa el
 * dashboard, que no se renderiza en servidor; el dia que algo publico lo lea,
 * esto ya es seguro.
 */
export function useSesion() {
  const perfil = useState<Perfil | null>('auth:perfil', () => null)
  return { perfil }
}
