# _template

Molde para copiar al crear un modulo nuevo:

```sh
cp -r internal/modules/_template internal/modules/pedidos
```

Despues: renombrar el paquete, ajustar `Name()`, `Permissions()` y `Routes()`,
escribir la primera migracion, y agregarlo a `internal/app/modules.go` **despues
de sus dependencias** — ese orden es el orden en que corren las migraciones.

El guion bajo del nombre es a proposito: `go build ./...` ignora los directorios
que empiezan con `_`, asi que el molde no entra en la compilacion del proyecto
ni en la cobertura. La prueba de limites lo compila aparte para que no se pudra.

Los ocho pasos completos estan en `docs/11-forks.md`.
