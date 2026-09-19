#!/usr/bin/env bash
# Prueba de scripts/enlazar-starter.sh y de "Traer mejoras del starter": arma un
# starter de mentira con historia y un fork de plantilla (un commit, sin
# historia comun), en un temporal. Nunca toca el checkout ni la red.
#
#   ./scripts/enlazar-starter.test.sh
set -euo pipefail

raiz=$(git rev-parse --show-toplevel)
viejo="go""-starter"

# Como rename.test.sh: la prueba arma el starter con el arbol de este repo y
# renombra, asi que en un fork ya renombrado no tiene sobre que correr. Sin esta
# salida, el CI de todo fork se pondria rojo (el defecto de HU-016).
if ! git -C "$raiz" grep -q -e "$viejo" -- ':!docs'; then
  echo "enlazar-starter.test: este repositorio ya es un fork renombrado: la prueba es del starter"
  exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

g() { git -c user.email=prueba@fork -c user.name=prueba -c init.defaultBranch=main "$@"; }
# Cada repo de la prueba con su identidad: los scripts hacen commits (el enlace)
# y en el CI no hay una global.
nuevo() { (cd "$1" && g init -q && git config user.email prueba@fork && git config user.name prueba); }
fallos=0
ok() { echo "ok   $1"; }
mal() { echo "MAL  $1"; fallos=$((fallos + 1)); }

# El starter: el arbol de HEAD con los scripts de trabajo, y despues un commit
# mas, para que su main de hoy NO sea el commit del que nace el fork.
s="$tmp/starter"
mkdir "$s"
git -C "$raiz" archive HEAD | tar -x -C "$s"
cp "$raiz"/scripts/*.sh "$s/scripts/"
nuevo "$s"
(cd "$s" && g add -A && g commit -qm "A: del que nace el fork")
origen=$(git -C "$s" rev-parse HEAD)

# El fork de plantilla: el mismo arbol, un solo commit, sin historia comun.
f="$tmp/fork"
mkdir "$f"
git -C "$s" archive HEAD | tar -x -C "$f"
nuevo "$f"
(cd "$f" && g add -A && g commit -qm "Initial commit")

echo "B: una linea nueva del starter" >>"$s/docs/11-forks.md"
(cd "$s" && g commit -qam "B: mejora del starter despues de que nacio el fork")

limpio() { [ -z "$(git -C "$f" status --porcelain)" ]; }
enlazar() { (cd "$f" && ./scripts/enlazar-starter.sh "$@") >"$tmp/salida" 2>&1; }

# Sin enlace, git se niega: es el problema que resuelve el script.
(cd "$f" && g remote add starter "$s" && g fetch -q starter)
if (cd "$f" && g merge -q starter/main) >"$tmp/salida" 2>&1; then
  mal "sin enlace, el merge paso"
else
  grep -q "unrelated histories" "$tmp/salida" && ok "sin enlace: refusing to merge unrelated histories" || mal "sin enlace fallo por otra cosa: $(cat "$tmp/salida")"
fi
(cd "$f" && g merge --abort 2>/dev/null || true; g remote remove starter)

echo x >>"$f/README.md"
if enlazar "$s"; then mal "enlazo con el arbol sucio"; else ok "se niega con el arbol sucio"; fi
git -C "$f" checkout -q README.md

if enlazar "$s"; then ok "enlaza: $(head -1 "$tmp/salida")"; else mal "no enlazo: $(cat "$tmp/salida")"; fi

base=$(git -C "$f" merge-base HEAD starter/main)
if [ "$base" = "$origen" ]; then ok "enlaza con el commit del que nacio, no con el main de hoy"; else mal "el ancestro comun es $base, se esperaba $origen"; fi
if [ "$(git -C "$f" rev-parse HEAD^{tree})" = "$(git -C "$f" rev-parse HEAD^1^{tree})" ]; then ok "el enlace no cambia ningun archivo"; else mal "el enlace cambio archivos"; fi

if enlazar && grep -q "ya esta enlazado" "$tmp/salida" && limpio; then ok "la segunda vez no hace nada"; else mal "la segunda vez: $(cat "$tmp/salida")"; fi

# Renombrar y traer la mejora B: entra sola, sin conflictos.
(cd "$f" && ./scripts/rename.sh tienda github.com/acme/tienda >/dev/null && g add -A && g commit -qm renombrar)
if grep -q "go""-starter.git" "$f/scripts/enlazar-starter.sh" 2>/dev/null || grep -qF '"https://github.com/ElYares/go""-starter.git"' "$f/scripts/enlazar-starter.sh"; then
  ok "el renombre no toca la URL del starter"
else
  mal "el renombre cambio la URL del starter: $(grep -n 'url=' "$f/scripts/enlazar-starter.sh")"
fi
if (cd "$f" && g merge -q --no-edit starter/main) >"$tmp/salida" 2>&1; then
  grep -q "B: una linea nueva del starter" "$f/docs/11-forks.md" && ok "trae la mejora B sin conflictos" || mal "el merge paso pero B no llego"
else
  mal "traer B fallo: $(cat "$tmp/salida")"
fi

# C: el starter agrega una linea con su nombre. Llega sin renombrar, y correr
# rename.sh otra vez la arregla.
mkdir -p "$s/api/internal/app"
printf 'package app\n\n// import "github.com/elyares/%s/api/internal/platform/httpx"\n' "$viejo" >"$s/api/internal/app/nuevo_del_starter.go"
(cd "$s" && g add -A && g commit -qm "C: el starter agrega una linea con su nombre")
(cd "$f" && g fetch -q starter && g merge -q --no-edit starter/main) >/dev/null 2>&1 || mal "traer C fallo"
if git -C "$f" grep -q "$viejo" -- ':!docs'; then ok "C llega con el nombre del starter, como se esperaba"; else mal "C no trajo el nombre del starter: la prueba no prueba nada"; fi
(cd "$f" && ./scripts/rename.sh tienda github.com/acme/tienda >/dev/null && g add -A && g commit -qm "renombrar lo que trajo C")
if git -C "$f" grep -q "$viejo" -- ':!docs'; then mal "tras renombrar otra vez sigue: $(git -C "$f" grep -n "$viejo" -- ':!docs' | head -2)"; else ok "rename.sh otra vez renombra lo que trajo el merge"; fi

# Enlazar DESPUES de renombrar, como un fork que nacio antes de este script
# (ElYares/prueba-fork): el enlace no puede traer el arbol del starter encima.
f2="$tmp/fork-renombrado"
mkdir "$f2"
git -C "$s" archive "$origen" | tar -x -C "$f2"
nuevo "$f2"
(cd "$f2" && g add -A && g commit -qm "Initial commit" && ./scripts/rename.sh viejo github.com/acme/viejo >/dev/null && g add -A && g commit -qm renombrar)
if (cd "$f2" && ./scripts/enlazar-starter.sh "$s") >"$tmp/salida" 2>&1 \
  && [ "$(git -C "$f2" rev-parse HEAD^{tree})" = "$(git -C "$f2" rev-parse HEAD^1^{tree})" ] \
  && ! git -C "$f2" grep -q "$viejo" -- ':!docs'; then
  ok "enlaza un fork ya renombrado sin traer el arbol del starter"
else
  mal "enlazar despues de renombrar: $(tail -3 "$tmp/salida")"
fi

# Un repo que no nacio de este starter: no se enlaza a ciegas.
o="$tmp/otro"
mkdir "$o"
cp -r "$s/scripts" "$o/"
echo "otro proyecto" >"$o/README.md"
nuevo "$o"
(cd "$o" && g add -A && g commit -qm "otro")
if (cd "$o" && ./scripts/enlazar-starter.sh "$s") >"$tmp/salida" 2>&1; then
  mal "enlazo un repo que no nacio del starter"
elif [ -z "$(git -C "$o" status --porcelain)" ] && ! git -C "$o" merge-base HEAD starter/main >/dev/null 2>&1; then
  ok "no enlaza un repo que no nacio del starter: $(head -1 "$tmp/salida")"
else
  mal "fallo con el repo ajeno pero dejo algo"
fi

if [ "$fallos" -gt 0 ]; then
  echo "$fallos caso(s) fallaron"
  exit 1
fi
echo "todos los casos pasan"
