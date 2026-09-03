import { fileURLToPath } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  // Vitest no pasa por Nuxt: compila los .vue con el plugin de Vite y nada mas.
  // El arnes completo de Nuxt (@nuxt/test-utils) levanta un servidor por archivo
  // y aqui no hace falta: los Base* no usan nada de Nuxt a proposito, para que
  // se puedan probar en milisegundos y para que un fork pueda sacarlos de aqui.
  //
  // El precio: los auto-imports de Nuxt NO existen en las pruebas. Por eso cada
  // componente de shared/ui/ importa `ref`, `computed` y compania desde 'vue' de
  // forma explicita. Si alguno se apoya en el auto-import, su prueba lo caza.
  plugins: [vue()],

  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./app', import.meta.url)),
    },
  },

  test: {
    // Por omision, node: las pruebas de logica pura no necesitan DOM y arrancan
    // mas rapido sin el. Las de componentes piden el suyo archivo por archivo
    // con `// @vitest-environment happy-dom` en la primera linea.
    environment: 'node',
    include: ['test/**/*.spec.ts', 'app/**/*.spec.ts'],
  },
})
