#!/usr/bin/env bash
# Crea un modulo de dominio desde api/internal/modules/_template y lo registra:
# los pasos 1, 2 y 5 de "Agregar un modulo de dominio" en docs/11-forks.md.
#
#   ./scripts/nuevo-modulo.sh pedidos
#
# Toca tres cosas y nada mas: la carpeta nueva, app/modules.go (su import y su
# linea del registro) y la lista esperada de app/modules_test.go, que existe
# para que agregar un modulo sea una decision visible en el diff.
#
# Lo que sigue siendo a mano: la migracion de verdad, el contrato, el frontend
# y el checklist de docs/04-reglas-de-crud.md.
set -euo pipefail

falla() {
  echo "nuevo-modulo: $*" >&2
  exit 1
}

[ $# -eq 1 ] || {
  echo "uso: ./scripts/nuevo-modulo.sh <nombre>   (por ejemplo: pedidos)" >&2
  exit 2
}
nombre=$1

# Es un nombre de paquete de Go, un prefijo de permisos (pedidos.read) y un
# prefijo de tablas (pedidos_cosas): minusculas y digitos, empezando por letra.
if ! [[ $nombre =~ ^[a-z][a-z0-9]{1,29}$ ]]; then
  falla "\"$nombre\" no sirve: minusculas y digitos, empezando por letra, de 2 a 30 caracteres (sin guiones: es un paquete de Go)"
fi

# Palabras reservadas de Go y nombres que ya significan algo en el proyecto.
case " break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var app platform plantilla " in
  *" $nombre "*) falla "\"$nombre\" es una palabra reservada de Go o un nombre del proyecto" ;;
esac

raiz=$(git rev-parse --show-toplevel 2>/dev/null) || falla "no estoy dentro de un repositorio git"
cd "$raiz/api"

plantilla=internal/modules/_template
destino=internal/modules/$nombre
registro=internal/app/modules.go
prueba=internal/app/modules_test.go

[ -d "$plantilla" ] || falla "no encuentro $plantilla"
[ ! -e "$destino" ] || falla "ya existe api/$destino"

modulo_go=$(awk '$1 == "module" { print $2; exit }' go.mod)
[ -n "$modulo_go" ] || falla "no encuentro la ruta del modulo en api/go.mod"

# Los anclajes, comprobados ANTES de escribir nada: si un fork los movio, el
# script sale sin dejar el proyecto a medias.
grep -q "^	\"$modulo_go/internal/modules/" "$registro" || falla "no encuentro los imports de modulos en api/$registro"
grep -q '^		// catalog.New(...),' "$registro" || falla "no encuentro la linea '// catalog.New(...),' en api/$registro: agrega el modulo a mano"
grep -q '^	esperado := \[\]string{' "$prueba" || falla "no encuentro 'esperado := []string{' en api/$prueba"

cp -R "$plantilla" "$destino"
rm -f "$destino/README.md"

# El paquete, Name(), los permisos, la tabla y la ruta. La ruta pasa de /cosas a
# /<nombre>: dos modulos nacidos del molde chocarian en /cosas.
find "$destino" -type f \( -name '*.go' -o -name '*.sql' \) -print0 |
  N="$nombre" xargs -0 perl -pi -e '
    s{^// Package plantilla es el molde de un modulo\. No se registra en modules\.go\.$}{// Package $ENV{N} nacio de _template con scripts/nuevo-modulo.sh. Describe aqui su dominio.};
    s{"/cosas"}{"/$ENV{N}"}g;
    s{plantilla}{$ENV{N}}g;
  '

# El import en su lugar alfabetico: gofmt ordena los imports y el paso de
# formato del CI fallaria con uno fuera de orden.
M="$modulo_go" N="$nombre" perl -0pi -e '
  my $nuevo = "\t\"$ENV{M}/internal/modules/$ENV{N}\"\n";
  s{((?:^\t"\Q$ENV{M}\E/internal/modules/[^"]+"\n)+)}{ join "", sort(split(/(?<=\n)/, $1), $nuevo) }me;
' "$registro"

N="$nombre" perl -pi -e 's{^(\t\t// catalog\.New\(\.\.\.\),)}{\t\t$ENV{N}.New(),\n$1}' "$registro"
N="$nombre" perl -pi -e 's{^(\tesperado := \[\]string\{.*)\}$}{$1, "$ENV{N}"\}}' "$prueba"

cat <<FIN
nuevo-modulo: api/$destino creado y registrado en api/$registro

Sigue, como docs/11-forks.md:
  - la migracion de verdad en api/$destino/migrations/0001_inicial.sql
  - las operaciones en api/openapi.yaml, con el tag $nombre, y regenerar
  - el checklist de docs/04-reglas-de-crud.md, seccion 8
  - en web: web/app/modules/$nombre/ y su entrada en modules/admin/secciones.ts
FIN
