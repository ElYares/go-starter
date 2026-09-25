#!/usr/bin/env bash
# Prueba de scripts/nuevo-modulo.sh sobre una copia del repositorio: nunca toca
# el checkout de quien la corre (en desarrollo, el api recargaria el modulo de
# prueba y lo migraria en la base local). Sale con 1 si algun caso falla.
#
#   ./scripts/nuevo-modulo.test.sh
#   NUEVO_MODULO_DEJAR_COPIA=/tmp/copia ./scripts/nuevo-modulo.test.sh
set -euo pipefail

raiz=$(git rev-parse --show-toplevel)
# Cada commit o fetch deja un `git maintenance run --auto --detach` que sigue
# reempaquetando .git despues de que el comando vuelve (git 2.55: repack
# geometrico). Si todavia escribe cuando la trampa corre `rm -rf`, sale con
# "Directory not empty" y la prueba falla con todos los casos en verde (visto en
# el job fork del CI, PR #32). Con la variable, lo hereda todo git que se corra.
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=maintenance.auto GIT_CONFIG_VALUE_0=false

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

copia="$tmp/repo"
mkdir "$copia"
git -C "$raiz" archive HEAD | tar -x -C "$copia"
cp "$raiz/scripts/nuevo-modulo.sh" "$raiz/scripts/quitar-modulo.sh" "$copia/scripts/"
(
  cd "$copia"
  git init -q
  git -c user.email=prueba@fork -c user.name=prueba add -A
  git -c user.email=prueba@fork -c user.name=prueba commit -qm inicial
)

fallos=0
ok() { echo "ok   $1"; }
mal() { echo "MAL  $1"; fallos=$((fallos + 1)); }
limpio() { [ -z "$(git -C "$copia" status --porcelain)" ]; }
correr() { (cd "$copia" && ./scripts/nuevo-modulo.sh "$@") >"$tmp/salida" 2>&1; }

for nombre in "" "Pedidos" "mis-pedidos" "mis_pedidos" "1pedidos" "p" "media" "identity" "type" "app" "plantilla"; do
  if correr "$nombre"; then
    mal "acepto \"$nombre\""
  elif ! limpio; then
    mal "\"$nombre\" fallo pero cambio archivos: $(git -C "$copia" status --porcelain | tr '\n' ' ')"
  else
    ok "rechaza \"$nombre\": $(head -1 "$tmp/salida")"
  fi
done

if correr; then mal "acepto sin argumentos"; else ok "pide el nombre"; fi

if correr pedidos; then ok "crea pedidos"; else mal "fallo el caso normal: $(cat "$tmp/salida")"; fi

# Solo toca su carpeta, el registro y la lista esperada de la prueba.
cambios=$(git -C "$copia" status --porcelain | LC_ALL=C sort | tr '\n' ' ')
esperado=" M api/internal/app/modules.go  M api/internal/app/modules_test.go ?? api/internal/modules/pedidos/ "
if [ "$cambios" = "$esperado" ]; then ok "solo toca su carpeta, el registro y la prueba"; else mal "toco: $cambios"; fi

d="$copia/api/internal/modules/pedidos"
m=$(awk '$1 == "module" { print $2; exit }' "$copia/api/go.mod")
while IFS='|' read -r archivo patron que; do
  if grep -qE -- "$patron" "$copia/api/$archivo"; then ok "$que"; else mal "$que: no hay /$patron/ en $archivo"; fi
done <<LISTA
internal/modules/pedidos/module.go|^package pedidos$|el paquete es pedidos
internal/modules/pedidos/module.go|return "pedidos"|Name() es pedidos
internal/modules/pedidos/module.go|Key: "pedidos\.read"|los permisos son pedidos.*
internal/modules/pedidos/module.go|"/pedidos", m\.listar|la ruta es /pedidos
internal/modules/pedidos/migrations/0001_inicial.sql|create table pedidos_cosas|la tabla es pedidos_cosas
internal/app/modules.go|^		pedidos\.New\(\),$|esta en el registro
internal/app/modules.go|^	"$m/internal/modules/pedidos"$|esta importado
internal/app/modules_test.go|"content", "pedidos"\}|esta en la lista esperada
LISTA

if grep -rq plantilla "$d"; then mal "queda 'plantilla' en el modulo: $(grep -rn plantilla "$d" | head -2)"; else ok "no queda 'plantilla'"; fi
if [ -e "$d/README.md" ]; then mal "se copio el README del molde"; else ok "sin el README del molde"; fi

# Los imports de modulos quedan en orden: gofmt los ordena y el CI lo exige.
imports=$(grep "^	\"$m/internal/modules/" "$copia/api/internal/app/modules.go")
if [ "$imports" = "$(printf '%s\n' "$imports" | LC_ALL=C sort)" ]; then ok "imports en orden"; else mal "imports fuera de orden:"$'\n'"$imports"; fi

# Quitarlo deja el arbol EXACTAMENTE como estaba: nada del modulo quedo
# fuera de lo que el script sabe deshacer.
if (cd "$copia" && ./scripts/quitar-modulo.sh pedidos) >"$tmp/salida" 2>&1 && limpio; then
  ok "quitarlo deja el arbol como estaba"
else
  mal "quitar pedidos: $(cat "$tmp/salida") $(git -C "$copia" status --porcelain | tr '\n' ' ')"
fi

# Un modulo del starter cableado con otros no se quita con el script.
if (cd "$copia" && ./scripts/quitar-modulo.sh media) >"$tmp/salida" 2>&1; then
  mal "quito media, que settings usa por su puerto"
elif limpio; then
  ok "no quita media: $(head -1 "$tmp/salida")"
else
  mal "fallo con media pero cambio archivos"
fi
if (cd "$copia" && ./scripts/quitar-modulo.sh noexiste) >/dev/null 2>&1; then mal "quito uno que no existe"; else ok "no quita uno que no existe"; fi

correr pedidos
# Dos veces el mismo: ya existe.
git -C "$copia" -c user.email=prueba@fork -c user.name=prueba add -A
git -C "$copia" -c user.email=prueba@fork -c user.name=prueba commit -qm pedidos
if correr pedidos; then mal "creo pedidos dos veces"; elif limpio; then ok "no lo crea dos veces"; else mal "la segunda vez cambio archivos"; fi

if [ -n "${NUEVO_MODULO_DEJAR_COPIA:-}" ]; then
  rm -rf "$NUEVO_MODULO_DEJAR_COPIA"
  cp -r "$copia" "$NUEVO_MODULO_DEJAR_COPIA"
  echo "copia con pedidos en $NUEVO_MODULO_DEJAR_COPIA"
fi

if [ "$fallos" -gt 0 ]; then
  echo "$fallos caso(s) fallaron"
  exit 1
fi
echo "todos los casos pasan"
