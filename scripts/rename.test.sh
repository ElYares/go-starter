#!/usr/bin/env bash
# Prueba de scripts/rename.sh sobre una copia del repositorio: nunca toca el
# checkout de quien la corre. Sale con 1 si algun caso falla.
#
#   ./scripts/rename.test.sh
#
# La copia sale de `git archive HEAD`, asi que prueba lo commiteado: lo que
# haya sin commitear no entra.
set -euo pipefail

raiz=$(git rev-parse --show-toplevel)
viejo="go""-starter"

# En un fork ya renombrado no queda nada que renombrar, y los casos de abajo
# —que buscan el nombre del starter— no tienen sobre que correr. Sin esta
# salida, el primer push de todo fork dejaba su CI en rojo (visto en la corrida
# de CU-007, en ElYares/prueba-fork).
if ! git -C "$raiz" grep -q -e "$viejo" -- ':!docs'; then
  echo "rename.test: este repositorio ya es un fork renombrado (no dice $viejo fuera de docs/): no hay renombre que probar"
  exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

copia="$tmp/fork"
mkdir "$copia"
git -C "$raiz" archive HEAD | tar -x -C "$copia"
# El script de la copia es el del arbol de trabajo: se prueba el que se esta
# escribiendo, aunque todavia no este commiteado.
cp "$raiz/scripts/rename.sh" "$copia/scripts/rename.sh"
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
correr() { (cd "$copia" && ./scripts/rename.sh "$@") >"$tmp/salida" 2>&1; }

# Un nombre o un modulo que no sirven: sale con error y no toca nada.
for args in \
  "|github.com/acme/x" \
  "Mi-Tienda|github.com/acme/x" \
  "mi tienda|github.com/acme/x" \
  "-tienda|github.com/acme/x" \
  "tienda-|github.com/acme/x" \
  "$viejo|github.com/acme/x" \
  "tienda|acme/x" \
  "tienda|github.com/acme/x/api" \
  "tienda|https://github.com/acme/x"; do
  nombre=${args%%|*}
  modulo=${args#*|}
  if correr "$nombre" "$modulo"; then
    mal "acepto \"$nombre\" \"$modulo\""
  elif ! limpio; then
    mal "\"$nombre\" \"$modulo\" fallo pero cambio archivos"
  else
    ok "rechaza \"$nombre\" \"$modulo\": $(head -1 "$tmp/salida")"
  fi
done

if correr solo-un-argumento; then mal "acepto un solo argumento"; else ok "pide los dos argumentos"; fi

# Con cambios sin commitear no arranca: el renombre se revisa con git diff.
echo x >>"$copia/README.md"
if correr mi-tienda github.com/acme/mi-tienda; then
  mal "renombro con el arbol sucio"
else
  ok "se niega con el arbol sucio"
fi
git -C "$copia" checkout -q README.md

# El caso normal.
if correr mi-tienda github.com/acme/mi-tienda; then
  ok "renombra: $(head -1 "$tmp/salida")"
else
  mal "fallo el caso normal: $(cat "$tmp/salida")"
fi

quedan=$(git -C "$copia" grep -n -e "$viejo" -- ':!docs' || true)
if [ -z "$quedan" ]; then ok "no queda $viejo fuera de docs/"; else mal "queda $viejo:"$'\n'"$quedan"; fi

if grep -qx 'module github.com/acme/mi-tienda/api' "$copia/api/go.mod"; then
  ok "el modulo Go es github.com/acme/mi-tienda/api"
else
  mal "go.mod dice: $(head -1 "$copia/api/go.mod")"
fi

for esperado in \
  "web/nuxt.config.ts:const DOMAIN = 'mi-tienda.localhost'" \
  ".devherd.yml:  domain: mi-tienda.localhost" \
  "api/cmd/seed/main.go:	emailPorOmision    = \"superadmin@mi-tienda.localhost\"" \
  "web/package.json:  \"name\": \"mi-tienda-web\","; do
  archivo=${esperado%%:*}
  linea=${esperado#*:}
  if grep -qxF "$linea" "$copia/$archivo"; then ok "$archivo: $linea"; else mal "$archivo no tiene: $linea"; fi
done

if [ -n "$(git -C "$copia" diff --stat -- docs)" ]; then mal "toco docs/"; else ok "docs/ intacto"; fi

# Correrlo otra vez: nada que hacer, nada que cambie.
git -C "$copia" -c user.email=prueba@fork -c user.name=prueba commit -qam renombrado
if correr otra github.com/acme/otra && grep -q "no queda nada" "$tmp/salida" && limpio; then
  ok "la segunda vez no hace nada"
else
  mal "la segunda vez: $(cat "$tmp/salida")"
fi

# Para quien quiera compilar o levantar el resultado.
if [ -n "${RENAME_DEJAR_COPIA:-}" ]; then
  rm -rf "$RENAME_DEJAR_COPIA"
  cp -r "$copia" "$RENAME_DEJAR_COPIA"
  echo "copia renombrada en $RENAME_DEJAR_COPIA"
fi

if [ "$fallos" -gt 0 ]; then
  echo "$fallos caso(s) fallaron"
  exit 1
fi
echo "todos los casos pasan"
