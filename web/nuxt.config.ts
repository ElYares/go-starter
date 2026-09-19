import { fileURLToPath } from 'node:url'

// El dominio local lo sirve devherd a traves del edge.
const DOMAIN = 'go-starter.localhost'

export default defineNuxtConfig({
  compatibilityDate: '2026-09-02',
  devtools: { enabled: true },

  css: ['~/assets/tokens/base.css'],

  // La decision de renderizado, en dos lineas. Ver Decision 004 del vault.
  //
  // Publico en servidor: un fork tipo tienda o landing vive de que Google y los
  // previews de WhatsApp vean contenido, y una SPA les entrega un div vacio.
  // Dashboard en cliente: nadie indexa un panel privado, y renderizarlo en
  // servidor obligaria a reenviar cookies de sesion en cada navegacion.
  routeRules: {
    '/**': { ssr: true },
    '/admin/**': { ssr: false },
    // El login tampoco: no hay nada que indexar, y un formulario renderizado en
    // servidor se puede enviar ANTES de hidratar. Ese envio es el nativo del
    // navegador —un GET a la misma URL— y no pasa por el cliente ni por el CSRF.
    '/login': { ssr: false },
    // Igual que el login: un formulario que no hay que indexar, y que no se
    // envie antes de hidratar.
    '/recuperar-contrasena': { ssr: false },
  },

  runtimeConfig: {
    // La que usa el SSR: red interna del compose. Desde el contenedor de Nuxt,
    // go-starter.localhost no existe.
    apiInternal: 'http://api:8080/api/v1',
    // El secreto con el que el SSR le dice al api a que visitante atiende
    // (HU-010). Solo del servidor: fuera de `public` no llega al navegador. Sin
    // default; lo pone NUXT_API_SSR_SECRET y el plugin de servidor exige que este.
    apiSsrSecret: '',
    public: {
      // La que usa el navegador: mismo origen, el edge la enruta.
      apiBase: '/api/v1',
    },
  },

  devServer: {
    // 0.0.0.0 porque corre dentro de un contenedor: atado a 127.0.0.1 seria
    // inalcanzable desde el edge.
    host: '0.0.0.0',
    port: 3000,
  },

  vite: {
    server: {
      // El detalle que muerde y no da error claro: sin clientPort el navegador
      // abre el WebSocket de HMR contra el 3000 del host, que no esta
      // publicado, y el HMR queda muerto en silencio.
      //
      // Va en 'ws' y no en 'hmr': Vite 8 renombro server.hmr.* a server.ws.*.
      // La forma vieja todavia funciona, pero avisa en cada arranque.
      // No se fija 'host': sin el, el cliente usa el hostname de la pagina, que
      // es justo lo que se quiere y sobrevive a un cambio de dominio.
      ws: {
        clientPort: 80,
      },
      // Hoy es redundante —Vite acepta cualquier host bajo .localhost— pero deja
      // de serlo el dia que el dominio no termine en .localhost, y entonces el
      // sintoma es un 403 sin explicacion.
      allowedHosts: [DOMAIN],
      // El catalogo de bloques vive en el api y el editor lo importa
      // (app/shared/blocks/catalogo.ts). Queda fuera de la raiz de web, y sin
      // esto Vite se niega a servirlo en desarrollo: el build funciona y el
      // editor en `nuxt dev` no carga.
      fs: {
        allow: [fileURLToPath(new URL('../api/internal/modules/content/bloques', import.meta.url))],
      },
    },
  },
})
