-- +goose Up

-- El pie de pagina, que la fase 0 no sembro. Con los esquemas por clave
-- (Decision 024), una clave que no existe no se puede crear con cualquier forma,
-- asi que el starter trae la suya con un valor que cumple su esquema.
--
-- `on conflict do nothing`: un fork que ya la hubiera creado a mano conserva la
-- suya.
insert into settings (key, value, is_public) values
    ('site.footer', '{"text":"Hecho con go-starter","links":[]}', true)
on conflict (key) do nothing;

-- +goose Down
delete from settings where key = 'site.footer';
