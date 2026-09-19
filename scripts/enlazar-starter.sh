#!/usr/bin/env bash
# Enlaza un fork nacido con `gh repo create --template` con la historia del
# starter, para que despues `git merge starter/main` traiga sus mejoras.
#
#   ./scripts/enlazar-starter.sh              # con el starter de GitHub
#   ./scripts/enlazar-starter.sh <url-o-ruta> # con otro (un fork de un fork)
#
# Un repo de plantilla nace con un solo commit y sin historia comun con el
# starter: `git merge starter/main` responde "refusing to merge unrelated
# histories", y forzandolo sale un conflicto por cada archivo renombrado.
#
# El enlace es un merge `-s ours` con el commit del starter del que nacio el
# fork: lo registra como ancestro sin cambiar ningun archivo. Tiene que ser ESE
# commit y no el main de hoy: enlazar con uno mas nuevo haria creer a git que el
# fork ya tiene los cambios de en medio, y los merges siguientes se los
# saltarian en silencio. Se encuentra solo: el primer commit del fork tiene el
# mismo arbol de archivos que el commit del starter del que salio.
#
# Se corre una vez, antes o despues de renombrar. Correrlo otra vez no hace nada.
set -euo pipefail

falla() {
  echo "enlazar-starter: $*" >&2
  exit 1
}

# Partido en dos: rename.sh no tiene que convertirlo en el nombre del fork.
url=${1:-"https://github.com/ElYares/go""-starter.git"}

raiz=$(git rev-parse --show-toplevel 2>/dev/null) || falla "no estoy dentro de un repositorio git"
cd "$raiz"

[ -z "$(git status --porcelain)" ] || falla "hay cambios sin commitear: commitea o descarta antes"

if git remote get-url starter >/dev/null 2>&1; then
  actual=$(git remote get-url starter)
  [ $# -eq 0 ] || [ "$actual" = "$url" ] || falla "el remoto starter ya apunta a $actual, no a $url"
else
  git remote add starter "$url"
fi
git fetch -q starter || falla "no pude traer el starter de $(git remote get-url starter)"

rama=$(git symbolic-ref -q --short refs/remotes/starter/HEAD 2>/dev/null || true)
rama=${rama:-starter/main}
git rev-parse -q --verify "$rama" >/dev/null || falla "no encuentro $rama en el remoto starter"

if base=$(git merge-base HEAD "$rama" 2>/dev/null); then
  echo "enlazar-starter: ya esta enlazado (ancestro comun $(git log -1 --format='%h %s' "$base"))"
  exit 0
fi

raices=$(git rev-list --max-parents=0 HEAD)
[ "$(printf '%s\n' "$raices" | wc -l | tr -d ' ')" = 1 ] || falla "este repo tiene mas de un commit raiz: enlazalo a mano"
arbol=$(git rev-parse "$raices^{tree}")

origen=""
for c in $(git rev-list "$rama"); do
  if [ "$(git rev-parse "$c^{tree}")" = "$arbol" ]; then
    origen=$c
    break
  fi
done
[ -n "$origen" ] || falla "ningun commit de $rama tiene el arbol del primer commit de este repo: no nacio de este starter, o nacio de una rama que no es $rama"

git merge -q -s ours --allow-unrelated-histories --no-edit \
  -m "Enlazar con el starter en $(git rev-parse --short "$origen")" "$origen"

cat <<FIN
enlazar-starter: enlazado con $(git log -1 --format='%h %s' "$origen")
El merge no cambio ningun archivo. Para traer mejoras, como sigue docs/11-forks.md:
  git fetch starter && git merge $rama
FIN
