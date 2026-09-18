#!/usr/bin/env bash
# Renombra un fork recien nacido del starter: el modulo Go, el dominio local,
# la cuenta de desarrollo, los titulos y todo lo que diga el nombre del starter.
#
#   ./scripts/rename.sh mi-tienda github.com/acme/mi-tienda
#
# Deja los cambios sin commitear para revisarlos con `git diff`. `docs/` no se
# toca: habla del starter como origen, y 11-forks.md usa su nombre para el
# remoto `starter` del que se traen mejoras.
#
# Portable a proposito: bash 3.2 (el de macOS) y perl, sin sed -i, que no
# acepta los mismos argumentos en GNU y en BSD.
set -euo pipefail

uso() {
  cat >&2 <<'USO'
uso: ./scripts/rename.sh <nombre> <modulo-go>

  nombre     minusculas, digitos y guiones, sin guion al inicio ni al final,
             hasta 40 caracteres. Es el dominio local (<nombre>.localhost),
             la cuenta de desarrollo y el nombre del paquete de web
  modulo-go  la ruta del modulo Go, sin /api: github.com/acme/mi-tienda

ejemplo: ./scripts/rename.sh mi-tienda github.com/acme/mi-tienda
USO
  exit 2
}

falla() {
  echo "rename: $*" >&2
  exit 1
}

[ $# -eq 2 ] || uso
nombre=$1
modulo=${2%/}

# Partidos en dos para que este archivo no aparezca en su propia busqueda, y
# para que correrlo otra vez no se renombre a si mismo.
viejo_nombre="go""-starter"
viejo_modulo="github.com/elyares/go""-starter"

# El nombre termina en un dominio (.localhost), en un correo y en un paquete
# npm: el patron es el comun a los tres.
if ! [[ $nombre =~ ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$ ]]; then
  falla "\"$nombre\" no sirve como nombre: solo minusculas, digitos y guiones, sin guion al inicio ni al final, hasta 40 caracteres"
fi
[ "$nombre" != "$viejo_nombre" ] || falla "\"$nombre\" es el nombre del starter: elige el del proyecto"

# Un dominio con punto en el primer segmento y al menos un segmento mas. Go
# acepta mas formas, pero una ruta que no se puede clonar no es la de un fork.
if ! [[ $modulo =~ ^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}(/[A-Za-z0-9._~-]+)+$ ]]; then
  falla "\"$modulo\" no parece la ruta de un modulo Go, por ejemplo github.com/acme/mi-tienda"
fi
case $modulo in
  */api) falla "el modulo va sin /api: el script lo agrega (\"${modulo%/api}\")" ;;
esac

raiz=$(git rev-parse --show-toplevel 2>/dev/null) || falla "no estoy dentro de un repositorio git"
cd "$raiz"

# Con el arbol limpio, `git diff` muestra exactamente lo que hizo el script, y
# `git checkout .` lo deshace entero.
if [ -n "$(git status --porcelain)" ]; then
  falla "hay cambios sin commitear: commitea o descarta antes, para poder revisar el renombre con git diff"
fi

archivos=$(git grep -l -e "$viejo_nombre" -- ':!docs' || true)
if [ -z "$archivos" ]; then
  echo "rename: no queda nada que renombrar: el proyecto ya no dice $viejo_nombre fuera de docs/"
  exit 0
fi

# Primero el modulo Go, que contiene al nombre; despues el nombre suelto.
# Las cadenas viajan por el entorno y \Q...\E las toma literales: un punto del
# modulo no es "cualquier caracter", y nada del nombre se interpreta.
git grep -lz -e "$viejo_nombre" -- ':!docs' |
  VM="$viejo_modulo" NM="$modulo" VN="$viejo_nombre" NN="$nombre" \
    xargs -0 perl -pi -e 's/\Q$ENV{VM}\E/$ENV{NM}/g; s/\Q$ENV{VN}\E/$ENV{NN}/g'

n=$(printf '%s\n' "$archivos" | wc -l | tr -d ' ')
cat <<FIN
rename: $n archivos renombrados a $nombre, modulo Go $modulo/api

Revisa con git diff y commitea. Despues, como sigue docs/11-forks.md:
  cp .env.example .env    # y llena los secretos
  devherd up && devherd proxy apply
FIN
