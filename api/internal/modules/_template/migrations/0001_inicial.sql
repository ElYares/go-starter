-- +goose Up

-- Numeracion local al modulo: 0001, 0002. No hay contador global, y cada modulo
-- lleva su propia tabla de versiones.
--
-- Las convenciones de columnas estan en docs/03-modelo-de-datos.md: uuid v7,
-- timestamptz siempre, enumerados con CHECK, y los cuatro campos de auditoria.
create table plantilla_cosas (
    id          uuid        primary key,
    nombre      text        not null,
    version     integer     not null default 1,
    created_at  timestamptz not null default now(),
    created_by  uuid,
    updated_at  timestamptz not null default now(),
    updated_by  uuid
);

-- +goose Down
drop table plantilla_cosas;
