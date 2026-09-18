# _template

Molde de un modulo nuevo. No se copia a mano: desde la raiz del repo,

```sh
./scripts/nuevo-modulo.sh pedidos
```

lo copia, renombra el paquete, `Name()`, los permisos, la tabla y la ruta, y lo
registra en `internal/app/modules.go` (al final, antes de `// catalog.New`: si
depende de otro que se registra despues, muevelo — el orden del registro es el
orden de las migraciones). `./scripts/quitar-modulo.sh pedidos` lo deshace.

Y el contrato, que va primero: declarar las operaciones en `api/openapi.yaml`
**con el tag del modulo**, copiar `openapi.cfg.yaml` de `settings/` cambiando
`package` e `include-tags`, agregar la linea `//go:generate`, correr
`go generate ./...` desde `api/` y afirmar `var _ ServerInterface = (*Module)(nil)`.
Sin el tag, las operaciones quedan fuera de lo generado y nadie avisa.
Ver `docs/05-contratos-api.md`.

El guion bajo del nombre es a proposito: `go build ./...` ignora los directorios
que empiezan con `_`, asi que el molde no entra en la compilacion del proyecto
ni en la cobertura. La prueba de limites lo compila aparte para que no se pudra.

Los ocho pasos completos estan en `docs/11-forks.md`.
