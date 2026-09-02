# go-starter

Base para proyectos web con **Go + Nuxt 4 + Postgres**, pensada para forkearse:
una landing publica que se edita desde un dashboard, y un nucleo de plataforma
(identidad, sesion, permisos, molde de CRUD, archivos) que el fork no toca.

```
/            landing publica, renderizada en servidor
/admin       dashboard de staff y admin, SPA
/api/v1      backend en Go
```

## Empezar

```sh
devherd up            # levanta el stack local
devherd proxy apply   # tras cualquier down
```

Local en `http://go-starter.localhost/`. Sin puertos publicados.

## Documentacion

Empieza por [`docs/INDEX.md`](docs/INDEX.md). El *por que* de cada decision vive
en el vault, no aqui.

## Licencia

MIT. Ver [LICENSE](LICENSE).
