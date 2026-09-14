-- +goose Up

-- La landing vive en la base, no en el codigo. Guardar crea una version;
-- publicar apunta a una. Ver la Decision 006 y docs/03-modelo-de-datos.md.
--
-- Las dos tablas van en una sola migracion porque se apuntan entre ellas: la
-- pagina a su version publicada, la version a su pagina. Partidas en dos, la
-- primera tendria que nacer sin la llave que la hace correcta.

-- La pagina es la direccion y el puntero. Lo que se edita —titulo, SEO,
-- bloques— vive en la version, para que editar no toque lo publicado y
-- revertir revierta todo, no solo los bloques.
create table pages (
    id                   uuid        primary key,
    -- El slug NO se versiona: es la direccion de la pagina. Cambiarlo cambia la
    -- URL publica al instante, y eso es lo que se espera de renombrar.
    slug                 text        not null,
    -- Nulo = nunca publicada, y el endpoint publico responde 404.
    published_version_id uuid,
    -- Concurrencia optimista sobre el borrador. Sube con cada guardado; publicar
    -- no la toca, para no mandar un 409 a quien esta editando.
    version              integer     not null default 1,
    created_at           timestamptz not null default now(),
    created_by           uuid,
    updated_at           timestamptz not null default now(),
    updated_by           uuid,

    constraint pages_slug_key unique (slug),
    -- El mismo patron que el contrato. Sin el CHECK, una fila escrita a mano
    -- con mayusculas o espacios produce una URL que el contrato no deja pedir.
    constraint pages_slug_formato check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' and length(slug) <= 80)
);

-- Las versiones son historia: se agregan, no se modifican. Por eso solo llevan
-- las columnas de creacion.
create table page_versions (
    id              uuid        primary key,
    page_id         uuid        not null references pages (id) on delete cascade,
    -- Local a la pagina: 1, 2, 3... No es global, porque "la version 412" de
    -- una pagina que se edito tres veces no le dice nada a quien la lee.
    number          integer     not null check (number >= 1),
    title           text        not null,
    seo_title       text,
    seo_description text,
    -- Siempre un array. La forma de cada bloque la valida el servidor contra el
    -- catalogo al guardar; la base solo garantiza que no sea otra cosa.
    blocks          jsonb       not null check (jsonb_typeof(blocks) = 'array'),
    note            text,
    created_at      timestamptz not null default now(),
    created_by      uuid,

    constraint page_versions_numero_key unique (page_id, number),
    -- Lo que permite la llave compuesta de abajo.
    constraint page_versions_de_su_pagina_key unique (page_id, id)
);

-- La version publicada tiene que ser de ESTA pagina. Una llave simple a
-- page_versions(id) dejaria publicar la version de otra pagina, y la landing
-- mostraria en /nosotros el contenido de /precios.
--
-- Con la version publicada nula la llave no se comprueba (MATCH SIMPLE), que es
-- justo "nunca publicada". Y como no tiene ON DELETE, una version publicada no
-- se puede borrar suelta; borrar la pagina entera si, porque la fila que la
-- apunta se va en la misma sentencia.
alter table pages
    add constraint pages_version_publicada_fk
    foreign key (id, published_version_id) references page_versions (page_id, id);

-- Contenido por omision del starter, como las claves de settings: un fork
-- recien clonado muestra una portada en vez de un 404. Ids fijos para que una
-- prueba o un seed puedan nombrarla. La firma el actor de sistema: nadie la
-- escribio.
--
-- Los bloques cumplen el catalogo del modulo. Lo comprueba una prueba: una
-- semilla que no pasa su propia validacion rebota el primer guardado de quien
-- la edite sin tocar nada.
insert into pages (id, slug, created_by, updated_by) values
    ('00000000-0000-7000-8000-0000000c0001', 'inicio',
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000');

insert into page_versions (id, page_id, number, title, seo_title, seo_description, blocks, note, created_by) values
    ('00000000-0000-7000-8000-0000000c1001', '00000000-0000-7000-8000-0000000c0001', 1,
     'Inicio', 'go-starter',
     'Landing publica editable desde un dashboard.',
     '[
        {"id": "hero", "type": "hero", "props": {
            "title": "Tu sitio, editable desde el dashboard",
            "subtitle": "Cambia este texto en /admin, publica, y la landing cambia sin un deploy.",
            "cta": {"label": "Entrar al dashboard", "href": "/admin"}
        }},
        {"id": "features", "type": "features", "props": {
            "title": "Lo que trae de base",
            "items": [
                {"title": "Contenido versionado", "text": "Guardar crea una version y publicar apunta a una. Revertir es publicar la anterior."},
                {"title": "Permisos por modulo", "text": "Quien edita puede no poder publicar."},
                {"title": "Hecho para forkearse", "text": "Una tienda, una landing o un panel, sin reescribir la base."}
            ]
        }},
        {"id": "texto", "type": "texto", "props": {
            "body": "Esta pagina la sembro la migracion del modulo content. Editala o borrala: es tuya."
        }}
     ]',
     'Pagina sembrada por la migracion',
     '00000000-0000-0000-0000-000000000000');

update pages
   set published_version_id = '00000000-0000-7000-8000-0000000c1001'
 where id = '00000000-0000-7000-8000-0000000c0001';

-- +goose Down
-- La llave circular se quita primero: sin eso, ninguna de las dos tablas se
-- puede borrar antes que la otra.
alter table pages drop constraint pages_version_publicada_fk;
drop table page_versions;
drop table pages;
