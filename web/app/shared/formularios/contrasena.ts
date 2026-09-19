/**
 * El generador de contrasenas del dashboard (HU-019): en el alta de una
 * cuenta, al asignar una temporal y al cambiar la propia.
 *
 * **Aleatoria de verdad y sin sesgo.** `crypto.getRandomValues` y no
 * `Math.random`, que no es para secretos. Y cada byte se usa solo si cae por
 * debajo del mayor multiplo del alfabeto que cabe en 256: `byte % 62` a secas
 * haria que los primeros caracteres salieran mas que los demas.
 *
 * Sin caracteres que se confunden al dictarla o copiarla a mano (`0/O`,
 * `1/l/I`): una temporal se la pasa una persona a otra, a veces leyendola.
 */
export const ALFABETO = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789-_.!?#'

/** Dieciseis: por encima del minimo de doce del api, y cabe en una linea. */
export const LARGO_POR_OMISION = 16

export type FuenteAleatoria = (bytes: Uint8Array) => Uint8Array

const delNavegador: FuenteAleatoria = (bytes) => crypto.getRandomValues(bytes)

export function generarContrasena(largo = LARGO_POR_OMISION, aleatorio: FuenteAleatoria = delNavegador): string {
  const tope = 256 - (256 % ALFABETO.length)
  let salida = ''
  while (salida.length < largo) {
    for (const byte of aleatorio(new Uint8Array(largo * 2))) {
      if (byte >= tope) continue
      salida += ALFABETO[byte % ALFABETO.length]
      if (salida.length === largo) break
    }
  }
  return salida
}
