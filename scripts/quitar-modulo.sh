#!/usr/bin/env bash
# Quita un modulo de dominio: su carpeta, su import y su linea en
# app/modules.go, y su nombre de la lista esperada de app/modules_test.go. Lo
# inverso de scripts/nuevo-modulo.sh: "Quitar un modulo" en docs/11-forks.md.
#
#   ./scripts/quitar-modulo.sh pedidos
#
# Si con eso el proyecto no compila, es un defecto del starter: alguien hizo
# que otro modulo importe este. El CI lo comprueba en el job `fork`.
#
# No borra las tablas: en una base con datos, eso es una migracion explicita.
set -euo pipefail

falla() {
  echo "quitar-modulo: $*" >&2
  exit 1
}

[ $# -eq 1 ] || {
  echo "uso: ./scripts/quitar-modulo.sh <nombre>   (por ejemplo: pedidos)" >&2
  exit 2
}
nombre=$1
[[ $nombre =~ ^[a-z][a-z0-9]{1,29}$ ]] || falla "\"$nombre\" no es un nombre de modulo"

raiz=$(git rev-parse --show-toplevel 2>/dev/null) || falla "no estoy dentro de un repositorio git"
cd "$raiz/api"

destino=internal/modules/$nombre
registro=internal/app/modules.go
prueba=internal/app/modules_test.go
modulo_go=$(awk '$1 == "module" { print $2; exit }' go.mod)

[ -d "$destino" ] || falla "no existe api/$destino"

# Todo comprobado antes de borrar nada. Un modulo que no se registra con
# "<nombre>.New(" en su propia linea —media, content— esta cableado con otros
# en app/modules.go, y quitarlo es a mano.
grep -q "^	\"$modulo_go/internal/modules/$nombre\"$" "$registro" || falla "no encuentro su import en api/$registro"
grep -q "^		$nombre\.New(.*),$" "$registro" || falla "no encuentro la linea '$nombre.New(...)' en api/$registro: este modulo se quita a mano"

rm -rf "$destino"
M="$modulo_go" N="$nombre" perl -ni -e '
  print unless $_ eq "\t\"$ENV{M}/internal/modules/$ENV{N}\"\n" or /^\t\t\Q$ENV{N}\E\.New\(.*\),$/;
' "$registro"
N="$nombre" perl -pi -e 's/, "\Q$ENV{N}\E"(?=[,}])// if /^\tesperado := \[\]string\{/' "$prueba"

echo "quitar-modulo: api/$destino quitado del registro. Sus tablas siguen en las bases que ya migraron."
