-- +goose Up

-- Las imagenes del sitio. La fila guarda metadatos y la llave; los bytes viven
-- detras de platform/storage. Ver docs/03-modelo-de-datos.md (Medios).
--
-- Una fila no se modifica nunca: el archivo ES su contenido. Por eso solo lleva
-- las columnas de creacion, igual que las versiones de una pagina.
create table media (
    id            uuid        primary key,
    -- La deduplicacion. Subir dos veces los mismos bytes devuelve esta fila, y
    -- dos subidas simultaneas terminan en una sola porque lo decide el indice,
    -- no un select previo.
    sha256        bytea       not null,
    -- Deducido de los BYTES, no del nombre ni del Content-Type del cliente.
    mime          text        not null,
    size_bytes    bigint      not null,
    width         integer     not null,
    height        integer     not null,
    -- El nombre de la PRIMERA subida. Una segunda con otro nombre no lo cambia.
    original_name text,
    -- Ruta dentro de storage.Store. Sale del hash, asi que dos filas nunca
    -- podrian apuntar a dos copias del mismo archivo.
    storage_key   text        not null,
    created_at    timestamptz not null default now(),
    created_by    uuid,

    constraint media_sha256_key unique (sha256),
    constraint media_sha256_largo check (length(sha256) = 32),
    -- Los mismos tres del contrato. Un tipo nuevo se agrega en los dos sitios.
    constraint media_mime_permitido check (mime in ('image/png', 'image/jpeg', 'image/webp')),
    constraint media_tamano check (size_bytes > 0),
    constraint media_dimensiones check (width > 0 and height > 0)
);

-- +goose Down
drop table media;
