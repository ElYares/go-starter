# 09 · Calidad

## Qué se prueba y con qué

| Nivel | Herramienta | Qué cubre |
|---|---|---|
| Unitario | `testing` | Reglas de negocio del `service`, con repos falsos |
| Integración | `testcontainers-go` + Postgres real | Repos, migraciones, transacciones |
| HTTP | `httptest` contra el router completo | Contrato: códigos, cuerpos, cookies, guards |
| Frontend | Vitest + Testing Library | Componentes, capa de API, los cuatro estados |
| Límites | prueba de arquitectura | Que las reglas de dependencia se cumplan |

Sin mocks de base. Un repo probado contra un mock prueba el mock.

## La prueba de límites

Las cuatro reglas de `01-arquitectura.md` se verifican, no se confían:

```go
func TestPlatformNoImportaModulos(t *testing.T) { … }
func TestModulosNoSeImportanEntreSi(t *testing.T) { … }  // excepción: identity
```

Recorre los imports de cada paquete con `go/packages` y falla nombrando el
import prohibido. Es lo que impide que la separación se erosione en el mes tres,
cuando alguien "solo necesita una función de allá".

## Verde que no prueba nada

Cuatro formas de que la suite mienta, las cuatro vistas en el proyecto hermano:

- **Un `PASS` no dice cuántas pruebas corrieron.** Leer el conteo, y romper el
  código a propósito para confirmar que la prueba muerde. Es la única forma de
  distinguir "verde" de "no se ejecutó"
- **Una migración que transforma datos pasa siempre contra una base vacía:** no
  hay filas que transformar. Hay que migrar hasta `N-1`, sembrar, y entonces
  aplicar
- **Una prueba de autorización que no verifica el caso negativo no prueba nada.**
  Por cada endpoint: sin sesión → `401`, con sesión sin permiso → `403`, recurso
  ajeno → `404`
- **En frontend, una prueba que pasa sola y falla en suite** es limpieza que
  falta entre montajes, no un bug del componente

## Definición de "hecho"

Un cambio está hecho cuando:

- [ ] `go test ./...` en verde, con el conteo revisado
- [ ] `go vet` y el linter sin avisos nuevos
- [ ] `npm run test:run`, `typecheck` y `build` en verde
- [ ] El checklist de [`04-reglas-de-crud.md`](04-reglas-de-crud.md) §8, si tocó un CRUD
- [ ] `openapi.yaml` actualizado y tipos regenerados, si cambió el contrato
- [ ] La documentación de `docs/` que quedó desactualizada, actualizada
- [ ] Una entrada de bitácora en el vault si hubo un hallazgo que duela repetir

## CI

GitHub Actions, tres trabajos en paralelo: `api` (test + vet + lint), `web`
(test + typecheck + build) y `contract` (que el spec y lo generado coincidan).

Detalles que ya costaron tiempo:

- **Pedir el run de CI exige nombrar el workflow:** `gh run list --branch main`
  puede devolver el de Dependabot. Va con `--workflow ci.yml`
- **Antes de subir una acción, preguntar cuál es la última versión.** Subir un
  major "por prudencia" nace desactualizado y trae el mismo trabajo otra vez:
  `gh api repos/actions/checkout/releases/latest -q .tag_name`

## Observabilidad

- `log/slog` en JSON, con `traceId` en cada línea
- `GET /healthz` y `GET /readyz` separados desde el primer día
- devherd Observe recoge los errores locales. El reporter del `api` se conecta
  en la fase 0, no "después": un starter sin errores visibles enseña a ignorarlos
