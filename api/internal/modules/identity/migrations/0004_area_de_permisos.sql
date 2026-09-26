-- +goose Up

-- La parte del sitio a la que pertenece un permiso, en palabras de quien usa el
-- dashboard ("Paginas", "Imagenes"). La vista de roles agrupa por ella.
--
-- El default vacio solo existe para las filas que ya estaban: la siembra corre
-- despues de las migraciones en cada arranque y le pone a cada permiso el area
-- que declara su modulo, y lo que nadie declara lo borra. Ninguna fila se queda
-- con el vacio mas alla de ese instante.
alter table permissions add column area text not null default '';

-- +goose Down
alter table permissions drop column area;
