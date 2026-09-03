-- +goose Up

-- La configuracion del sitio: lo que un fork cambia sin tocar codigo.
--
-- No es un cajon de sastre. Si algo tiene reglas propias, ciclo de vida o se
-- consulta con filtros, es una tabla, no un setting.
create table settings (
    key         text primary key,
    value       jsonb       not null,
    -- Que una clave sea publica es una decision explicita. El default es
    -- privado: se filtra en el servidor, nunca en el frontend, porque aqui
    -- viven tambien llaves de terceros y correos internos.
    is_public   boolean     not null default false,
    -- Concurrencia optimista. Sin esto, dos personas editando la marca desde el
    -- dashboard producen que la ultima en guardar borre el trabajo de la otra,
    -- en silencio y sin rastro.
    version     integer     not null default 1,
    created_at  timestamptz not null default now(),
    created_by  uuid,
    updated_at  timestamptz not null default now(),
    updated_by  uuid
);

-- El listado publico filtra por is_public en cada carga de la landing.
create index settings_publicas_idx on settings (key) where is_public;

-- Contenido por omision del starter, no datos de negocio: es lo que hace que un
-- fork recien clonado muestre algo coherente en vez de una landing vacia.
insert into settings (key, value, is_public) values
    ('site.brand', '{"name":"go-starter","tagline":"Landing publica editable desde un dashboard"}', true),
    ('site.nav',   '[{"label":"Inicio","href":"/"}]', true),
    ('site.theme', '{"accent":"#2f6df6"}', true);

-- +goose Down
drop table settings;
