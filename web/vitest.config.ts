import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    // Las pruebas de logica pura no necesitan DOM ni el arnes de Nuxt. Las de
    // componentes llegan en la fase 1, con su propio entorno.
    environment: 'node',
    include: ['test/**/*.spec.ts', 'app/**/*.spec.ts'],
  },
})
