package app

import "net/http"

// La UI de exploracion de la API. Vive en el MISMO origen que la API, detras
// del edge, y eso es lo que la hace util cuando llegue la sesion: las cookies
// HttpOnly viajan solas, sin copiar tokens a mano. Una herramienta externa
// contra otro origen no puede hacer eso.
//
// Solo existe con APP_ENV=dev y API_DOCS_ENABLED=true. Ver config.Load.
const scalarHTML = `<!doctype html>
<html>
  <head>
    <title>go-starter · API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="robots" content="noindex" />
  </head>
  <body>
    <div id="app"></div>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
    <script>
      Scalar.createApiReference('#app', {
        url: '/api/v1/openapi.yaml',
        theme: 'default',
      })
    </script>
  </body>
</html>
`

func (a *App) apiDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(scalarHTML))
}

func (a *App) openapiSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(a.spec)
}
