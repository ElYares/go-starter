#!/usr/bin/env bash
# Corre cmd/seed dentro del contenedor del api de ESTE checkout: migraciones,
# catalogo de permisos y el superadmin de desarrollo. Idempotente.
#
#   ./scripts/seed.sh
#
# El contenedor se encuentra por las etiquetas de compose, no por su nombre:
# devherd lo llama devherd-<carpeta>-<hash>-api-1, y ese nombre cambia de una
# maquina a otra y de un fork a otro.
set -euo pipefail

raiz=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "seed: no estoy dentro de un repositorio git" >&2; exit 1; }

api=$(docker ps -q \
  --filter "label=com.docker.compose.project.working_dir=$raiz" \
  --filter "label=com.docker.compose.service=api")

if [ -z "$api" ]; then
  echo "seed: no hay un contenedor del api corriendo para $raiz. Levanta el stack con: devherd up" >&2
  exit 1
fi

exec docker exec -w /workspace "$api" go run ./cmd/seed "$@"
