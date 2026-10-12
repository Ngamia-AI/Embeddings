-- Apps, keys and usage schema for the embeddings gateway.

create table if not exists apps (
  id uuid primary key,
  name text not null unique,
  status text not null default 'active',          -- active | disabled
  modalities text[] not null default '{text}',    -- text | image | audio | video
  rps int not null default 10,
  burst int not null default 20,
  media_rps int not null default 2,
  max_batch int not null default 32,
  max_media int not null default 8,
  created_at timestamptz not null default now()
);

create table if not exists api_keys (
  id uuid primary key,
  app_id uuid not null references apps(id),
  prefix text not null,                        -- first chars, for display
  key_hash bytea not null unique,              -- sha256(key)
  class text not null default 'live',          -- live | bulk
  status text not null default 'active',       -- active | revoked
  last_used_at timestamptz,
  created_at timestamptz not null default now(),
  revoked_at timestamptz
);

create index if not exists api_keys_app_id_idx on api_keys (app_id);

create table if not exists usage_events (
  id bigserial primary key,
  app_id uuid not null,
  key_id uuid not null,
  request_id text not null,
  inputs int not null,
  tokens int not null,
  images int not null default 0,
  audio_seconds numeric(8,2) not null default 0,
  video_seconds numeric(8,2) not null default 0,
  latency_ms int not null,
  status int not null,
  created_at timestamptz not null default now()
);
create index if not exists usage_events_app_created_idx on usage_events (app_id, created_at);

create table if not exists usage_daily (
  app_id uuid not null,
  day date not null,
  requests bigint not null default 0,
  inputs bigint not null default 0,
  tokens bigint not null default 0,
  images bigint not null default 0,
  audio_seconds numeric(12,2) not null default 0,
  video_seconds numeric(12,2) not null default 0,
  errors bigint not null default 0,
  primary key (app_id, day)
);
