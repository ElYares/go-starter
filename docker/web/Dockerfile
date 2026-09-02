# Imagen de desarrollo del frontend.
#
# El porque, igual que en docker/api: node_modules va en un volumen nombrado, no
# en el bind mount (montar el node_modules del host rompe los binarios nativos,
# glibc contra musl). Al crear ese volumen, Docker copia el contenido y el
# propietario del directorio que hay en la imagen; si no existe, lo crea de root
# y 'npm install' falla con EACCES al correr como usuario no root.
ARG NODE_VERSION=24
FROM node:${NODE_VERSION}-alpine

ARG UID=1000
ARG GID=1000

RUN mkdir -p /workspace/node_modules && chown -R ${UID}:${GID} /workspace

WORKDIR /workspace
