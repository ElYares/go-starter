-- +goose Up

-- HU-019. Una contrasena que asigno otra persona es temporal: quien la asigno
-- la conoce. Hasta que la cuenta elija la suya, Service.Actor no le resuelve
-- ningun permiso.
alter table users add column must_change_password boolean not null default false;

-- Lo que deja "¿Olvidaste tu contrasena?" en el login. El starter no manda
-- correos: la solicitud la ve quien tiene identity.user.password y entrega la
-- contrasena temporal por fuera.
create table password_reset_requests (
    id          uuid        primary key,
    user_id     uuid        not null references users (id) on delete cascade,
    created_at  timestamptz not null default now(),
    -- Para poder reconocer despues un pedido que no hizo la persona.
    ip          inet,
    user_agent  text,
    resolved_at timestamptz,
    resolved_by uuid
);

-- Una sola pendiente por cuenta. Es lo que hace que pedirlo diez veces —o que
-- alguien lo pida diez veces por otro— deje una fila y no diez, y lo que deja
-- escribir el pedido como un `insert ... on conflict do nothing`.
create unique index password_reset_requests_pendiente_idx
    on password_reset_requests (user_id) where resolved_at is null;

-- +goose Down
drop table password_reset_requests;
alter table users drop column must_change_password;
