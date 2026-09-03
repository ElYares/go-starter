-- +goose Up

-- citext es una extension, no un tipo de serie. Sin este create, la tabla de
-- usuarios no se puede crear y el error aparece a mitad de la migracion.
--
-- La alternativa —guardar `text` y comparar con lower()— falla el dia que
-- alguien escribe una consulta sin el lower(), y entonces Ana@casa.com y
-- ana@casa.com son dos cuentas. El tipo lo arregla de una vez, en la base.
create extension if not exists citext;

create table users (
    id            uuid        primary key,
    -- citext + unique: la comparacion insensible a mayusculas la hace el
    -- indice, asi que no hay forma de saltarsela desde el codigo.
    email         citext      not null unique,
    -- argon2id en formato PHC. NUNCA el texto plano, ni cifrado reversible:
    -- una fuga de la base no debe entregar contrasenas.
    password_hash text        not null,
    display_name  text        not null,
    enabled       boolean     not null default true,
    -- Marca al admin que crea `cmd/seed`. Es lo que permite que sus
    -- credenciales dejen de funcionar fuera de desarrollo aunque la fila
    -- sobreviva a un volcado copiado a otro entorno.
    dev_seed      boolean     not null default false,
    -- Concurrencia optimista: el CRUD de usuarios del dashboard llega en una
    -- fase posterior, y agregar la columna despues obliga a rellenarla.
    version       integer     not null default 1,
    created_at    timestamptz not null default now(),
    created_by    uuid,
    updated_at    timestamptz not null default now(),
    updated_by    uuid
);

-- Los cuatro roles del starter. Son contenido por omision, como las claves de
-- `settings`: un fork recien clonado tiene con que trabajar en vez de una tabla
-- vacia y un login que no lleva a ningun lado.
--
-- `superadmin` y `admin` no son el mismo rol con otro nombre. El primero
-- reparte poder —cuentas, roles, permisos— y por eso recibe TODO permiso
-- declarado, siempre. El segundo opera el sitio: recibe lo que ningun modulo
-- marco como sensible, y de ahi en adelante sus concesiones son datos que se
-- editan. Ver docs/03-modelo-de-datos.md.
--
-- Los ids son fijos y escritos a mano a proposito. Un uuid v7 generado aqui
-- seria distinto en cada entorno, y entonces un seed o una prueba no podria
-- nombrar el rol superadmin sin consultarlo antes.
create table roles (
    id   uuid primary key,
    key  text not null unique,
    name text not null
);

insert into roles (id, key, name) values
    ('00000000-0000-7000-8000-000000000001', 'superadmin', 'Superadministracion'),
    ('00000000-0000-7000-8000-000000000002', 'admin',      'Administracion'),
    ('00000000-0000-7000-8000-000000000003', 'staff',      'Equipo'),
    ('00000000-0000-7000-8000-000000000004', 'viewer',     'Solo lectura');

-- El catalogo de permisos. Esta tabla NO se llena aqui, y es el punto entero:
-- la siembra el arranque desde `Module.Permissions()` de cada modulo.
--
-- Una migracion que inserta permisos a mano se desincroniza el dia que se borra
-- el modulo que los inventaba: quedan filas nombrando algo que ya no existe, y
-- roles con permisos que no llevan a ninguna ruta.
create table permissions (
    key         text primary key,
    description text    not null,
    -- Lo marca el modulo que inventa el permiso, en su Permissions(). Aqui se
    -- guarda para que la pantalla de roles pueda avisar de lo que esta
    -- concediendo sin tener que preguntarle al codigo de Go.
    sensitive   boolean not null default false
);

create table role_permissions (
    role_id        uuid not null references roles (id) on delete cascade,
    -- El `on delete cascade` es lo que hace segura la reconciliacion del
    -- arranque: cuando un modulo desaparece, sus permisos se van y con ellos
    -- las concesiones que los nombraban.
    permission_key text not null references permissions (key) on delete cascade,
    primary key (role_id, permission_key)
);

create table user_roles (
    user_id uuid not null references users (id) on delete cascade,
    role_id uuid not null references roles (id) on delete cascade,
    primary key (user_id, role_id)
);

-- La sesion larga. Se guarda el SHA-256 del valor, nunca el valor: una fuga de
-- la base no debe entregar sesiones activas.
create table refresh_tokens (
    id          uuid        primary key,
    user_id     uuid        not null references users (id) on delete cascade,
    token_hash  bytea       not null,
    -- Lo que detecta el robo de un refresh: si llega uno ya rotado, su
    -- `replaced_by` dice que la cadena siguio sin el, y se tumba la familia
    -- entera. Ver docs/06-flujos.md.
    replaced_by uuid        references refresh_tokens (id),
    revoked_at  timestamptz,
    expires_at  timestamptz not null,
    user_agent  text,
    ip          inet,
    created_at  timestamptz not null default now()
);

-- Unico porque el hash ES la identidad del token al validarlo: dos filas con el
-- mismo hash harian ambiguo a cual pertenece la sesion que llega.
create unique index refresh_tokens_hash_idx on refresh_tokens (token_hash);

-- El listado que importa es "las sesiones vivas de este usuario", que es lo que
-- se recorre al cerrar sesion en todas partes.
create index refresh_tokens_vivos_idx on refresh_tokens (user_id) where revoked_at is null;

-- +goose Down
drop table refresh_tokens;
drop table user_roles;
drop table role_permissions;
drop table permissions;
drop table roles;
drop table users;
-- La extension no se borra: puede estar en uso por otra cosa en la misma base,
-- y un `drop extension` en cascada se llevaria columnas ajenas por delante.
