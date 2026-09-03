# _template

Molde para copiar al crear un modulo nuevo:

```sh
cp -r internal/modules/_template internal/modules/pedidos
```

Despues: renombrar el paquete, ajustar `Name()`, `Permissions()` y `Routes()`,
escribir la primera migracion, y agregarlo a `internal/app/modules.go` **despues
de sus dependencias** — ese orden es el orden en que corren las migraciones.

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
