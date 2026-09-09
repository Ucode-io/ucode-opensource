create type api_key_status_type as enum ('ACTIVE', 'INACTIVE');

create table if not exists api_keys (
    id uuid not null unique,
    status api_key_status_type not null default 'ACTIVE',
    name varchar not null default '',
    app_id varchar not null unique,
    app_secret varchar not null,
    role_id uuid not null,
    resource_environment_id uuid not null,
    created_at timestamp default now(),
    updated_at timestamp default now()
);