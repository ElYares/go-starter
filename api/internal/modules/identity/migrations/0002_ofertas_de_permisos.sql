-- +goose Up

-- Lo que la siembra ya le ofrecio a un rol, lo tenga hoy o no.
--
-- Es lo que distingue "al admin le quitaron este permiso" de "este permiso es
-- nuevo y nunca se le ofrecio". Sin esa distincion hay dos salidas y las dos son
-- malas: repartirle al admin en cada despliegue, y lo que se le quite desde el
-- dashboard vuelve solo; o repartirle una sola vez, y el permiso de cada modulo
-- que llegue despues se queda en el superadmin para siempre.
--
-- Quitar una concesion NO toca su oferta: por eso lo quitado queda quitado.
create table role_permission_offers (
    role_id        uuid not null references roles (id) on delete cascade,
    -- Si el modulo desaparece, la oferta se va con su permiso. Si vuelve, es un
    -- permiso nuevo otra vez y se ofrece de nuevo.
    permission_key text not null references permissions (key) on delete cascade,
    primary key (role_id, permission_key)
);

-- Una instalacion que ya existia: lo que el admin tiene hoy cuenta como
-- ofrecido, y lo que no tiene se le ofrecera en la siguiente siembra. Es la
-- lectura correcta porque hasta aqui no habia forma de quitarle un permiso al
-- admin salvo por SQL; quien lo hizo asi tendra que volver a quitarlo.
insert into role_permission_offers (role_id, permission_key)
select rp.role_id, rp.permission_key
  from role_permissions rp
  join roles ro on ro.id = rp.role_id
 where ro.key = 'admin';

-- +goose Down
drop table role_permission_offers;
