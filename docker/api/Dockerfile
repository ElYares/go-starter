# Imagen de desarrollo del backend.
#
# Existe por dos razones, las dos de permisos y las dos con el mismo sintoma
# (EACCES a mitad de un build que "deberia funcionar"):
#
# 1. 'air' tiene que estar instalado en la imagen. Instalarlo en cada arranque
#    sumaria medio minuto a cada 'devherd up'.
# 2. Las cachés de Go van en volumenes nombrados. Docker copia el contenido y el
#    PROPIETARIO del directorio que existe en la imagen al crear el volumen; si
#    el directorio no existe, lo crea de root y el contenedor —que corre con el
#    UID del host— no puede escribir en el.
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-alpine

ARG UID=1000
ARG GID=1000

# git lo necesita 'go install'; curl es para healthchecks legibles.
RUN apk add --no-cache git curl

RUN go install github.com/air-verse/air@latest

ENV GOPATH=/go \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/gocache \
    HOME=/tmp \
    CGO_ENABLED=0

RUN mkdir -p /go/pkg/mod /gocache /workspace \
    && chown -R ${UID}:${GID} /go /gocache /workspace

WORKDIR /workspace
