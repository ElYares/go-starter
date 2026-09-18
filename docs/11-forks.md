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

```sh
./scripts/nuevo-modulo.sh pedidos
```

Hace los pasos 1, 2 y 5: copia `_template` a `api/internal/modules/pedidos`,
renombra el paquete, `Name()`, los permisos (`pedidos.read`, `pedidos.write`), la
tabla y la ruta (`/pedidos`), y lo registra en `app/modules.go` —el import en su
orden, que gofmt exige— y en la lista esperada de `app/modules_test.go`. Valida
el nombre (es un paquete de Go: minúsculas y dígitos, sin guiones) antes de tocar
nada. El resto sigue siendo a mano:

1. ~~Copiar `_template`~~ (el script)
2. ~~Ajustar `module.go`: `Name`, `Permissions`, `Routes`~~ (el script, con los
   nombres del molde; lo propio del dominio es tuyo)
3. Escribir `migrations/0001_inicial.sql` con las convenciones de
   `03-modelo-de-datos.md`
4. Declarar en `ports.go` lo que necesite de otros módulos. **Ningún import de
   otro módulo** (salvo `identity`, por su `Actor`)
5. ~~Registrarlo en `app/modules.go`~~ (el script)
6. Agregarlo a `openapi.yaml` con el tag del módulo y regenerar los dos lados
7. Recorrer el checklist de `04-reglas-de-crud.md` §8
8. En el front: `web/app/modules/pedidos/` y su entrada en el shell del dashboard

## Quitar un módulo

```sh
./scripts/quitar-modulo.sh pedidos
go build ./... && npm run build
```

Borra la carpeta, su import y su línea del registro, y su nombre de la lista
esperada. Sus rutas de `openapi.yaml`, si las tenía, se quitan a mano y se
regenera. Un módulo que no se registra con `<nombre>.New(...)` en su propia
línea —`media`, `content`, cableados con otros en `app/modules.go`— el script no
lo toca: se quita a mano.

Si con eso algo más deja de compilar **es un defecto del starter, no del fork**:
significa que alguien rompió la regla de que un módulo no importa otro. Se
arregla arriba y se sube por PR.

**El CI recorre las dos recetas en cada PR** (job `fork`): agrega `pedidos`,
comprueba formato, build, vet, sus migraciones y sus permisos, y las pruebas; lo
quita, exige que el checkout quede exactamente como estaba, y vuelve a compilar
y probar. Si el molde se pudre o un módulo importa otro, falla ahí.

Nota sobre la base: borrar el código no borra las tablas de una base existente.
En un fork nuevo no hay nada que borrar; en uno con datos, la limpieza es una
migración explícita y deliberada. **En desarrollo, el api recarga y migra solo**:
agregar un módulo de prueba crea sus tablas en tu base local, y quitarlo no las
borra.

## Traer mejoras del starter

```sh
git remote add starter git@github.com:ElYares/go-starter.git
git fetch starter && git merge starter/main
```

Funciona mientras el fork respete la tabla de arriba. Los conflictos que salgan
son exactamente donde el fork se desvió, y esa información también es útil: si
un archivo de `platform/` conflictúa, ahí hay un parche que debió ser PR.
