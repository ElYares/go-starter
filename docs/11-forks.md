# 11 · Cómo se forkea

El starter existe para esto. Si forkear duele, el starter falló.

## Qué se toca y qué no

| Zona | En un fork |
|---|---|
| `internal/platform/**` | **No se toca.** Si un fork necesita cambiarla, es un cambio del starter |
| `internal/modules/identity` | Se extiende (campos de perfil), no se reescribe |
| `internal/modules/content`, `media`, `settings` | Se quedan. Son el valor del starter |
| `internal/modules/<tu dominio>` | Todo tuyo |
| `web/app/shared/blocks/**` | Se reemplazan por los bloques del proyecto |
| `web/app/assets/tokens/**` | **Lo primero que se cambia.** Aquí vive la piel |
| `web/app/modules/landing` | Se rediseña libremente |
| `web/app/modules/admin` | Se le agregan secciones, el shell se queda |

Regla para saber de qué lado cae algo: **si el arreglo sirve para todos los
forks, va al starter y se sube por PR; si es de este proyecto, se queda.** Un
fork que arrastra parches de plataforma no va a poder volver a traer mejoras.

## Nacer

```sh
gh repo create mi-tienda --template ElYares/go-starter --private --clone
cd mi-tienda
./scripts/rename.sh mi-tienda github.com/acme/mi-tienda   # y commitear el resultado
cp .env.example .env
# JWT_SIGNING_KEY no tiene default: openssl rand -base64 48
devherd up && devherd proxy apply
./scripts/seed.sh                      # migraciones, permisos y el superadmin
```

Lo que hace cada paso que no se ve:

- **`--template` exige que `ElYares/go-starter` esté marcado como template** en
  la configuración del repo en GitHub. Sin eso, `gh` falla. El fork nace sin la
  historia del starter; para traer mejoras después, ver abajo
- **`rename.sh`** cambia el nombre del starter en todo el proyecto salvo
  `docs/`: el módulo Go (`<modulo>/api`), el dominio local (`<nombre>.localhost`),
  la cuenta de desarrollo (`superadmin@<nombre>.localhost`), los títulos, la marca
  sembrada y el paquete de web. Pide el árbol limpio, para que `git diff` muestre
  exactamente lo que hizo; correrlo dos veces no hace nada. Sus casos se prueban
  con `./scripts/rename.test.sh`
- **`devherd proxy apply` pide `sudo`.** Sin él, `<nombre>.localhost` lo contesta
  el proxy compartido con un `200` vacío: parece que funciona y no llega al
  stack
- **`seed.sh`** encuentra el contenedor del api de este checkout por las
  etiquetas de compose. Entra en `/admin` con `superadmin@<nombre>.localhost` y
  `superadmin-de-desarrollo`, que solo autentican en desarrollo

Antes de escribir nada propio: cambiar los tokens y ver la landing con la marca
del proyecto. Es diez minutos y evita construir tres semanas sobre una identidad
prestada.

## Agregar un módulo de dominio

1. `cp -r internal/modules/_template internal/modules/orders`
2. Ajustar `module.go`: `Name`, `Permissions`, `Routes`
3. Escribir `migrations/0001_orders.sql` con las convenciones de
   `03-modelo-de-datos.md`
4. Declarar en `ports.go` lo que necesite de otros módulos. **Ningún import de
   otro módulo**
5. Registrarlo en `app/modules.go`, después de sus dependencias
6. Agregarlo a `openapi.yaml` y regenerar los dos lados
7. Recorrer el checklist de `04-reglas-de-crud.md` §8
8. En el front: `web/app/modules/orders/` y su entrada en el shell del dashboard

## Quitar un módulo

```sh
rm -rf api/internal/modules/catalog web/app/modules/catalog
# quitar su línea de app/modules.go
# quitar sus rutas de openapi.yaml y regenerar
go build ./... && npm run build
```

Si eso no basta —si algo más deja de compilar— **es un defecto del starter, no
del fork**: significa que alguien rompió la regla de que un módulo no importa
otro. Se arregla arriba y se sube por PR.

Nota sobre la base: borrar el código no borra las tablas de una base existente.
En un fork nuevo no hay nada que borrar; en uno con datos, la limpieza es una
migración explícita y deliberada.

## Traer mejoras del starter

```sh
git remote add starter git@github.com:ElYares/go-starter.git
git fetch starter && git merge starter/main
```

Funciona mientras el fork respete la tabla de arriba. Los conflictos que salgan
son exactamente donde el fork se desvió, y esa información también es útil: si
un archivo de `platform/` conflictúa, ahí hay un parche que debió ser PR.
