-- +goose Up
create extension if not exists pgcrypto;

do $$ begin
  if not exists (select 1 from pg_type where typname = 'doc_status') then
    create type doc_status as enum ('UPLOADED', 'PROCESSING', 'READY', 'FAILED');
  end if;

  if not exists (select 1 from pg_type where typname = 'job_status') then
    create type job_status as enum ('CREATED', 'RUNNING', 'COMPLETED', 'FAILED');
  end if;

  if not exists (select 1 from pg_type where typname = 'step_name') then
    create type step_name as enum ('OCR', 'LLM', 'NORMALIZE');
  end if;

  if not exists (select 1 from pg_type where typname = 'step_status') then
    create type step_status as enum ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED');
  end if;
end $$;

create table if not exists users (
  id uuid primary key default gen_random_uuid(),
  email text not null unique,
  password_hash text not null,
  created_at timestamptz not null default now()
);

create table if not exists refresh_sessions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(id) on delete cascade,
  refresh_hash text not null,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  revoked_at timestamptz null,
  user_agent text null,
  ip inet null
);
create index if not exists refresh_sessions_user_id_idx on refresh_sessions(user_id);
create index if not exists refresh_sessions_expires_idx on refresh_sessions(expires_at);

create table if not exists documents (
  id uuid primary key default gen_random_uuid(),
  owner_id uuid not null references users(id) on delete cascade,
  filename text not null,
  mime text not null,
  size bigint not null,
  checksum text not null,
  object_key text not null,
  status doc_status not null default 'UPLOADED',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists documents_owner_created_idx on documents(owner_id, created_at desc);
create index if not exists documents_owner_status_idx on documents(owner_id, status);

create table if not exists jobs (
  id uuid primary key default gen_random_uuid(),
  document_id uuid not null references documents(id) on delete cascade,
  owner_id uuid not null references users(id) on delete cascade,
  pipeline_version int not null default 1,
  status job_status not null default 'CREATED',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists jobs_document_idx on jobs(document_id);
create index if not exists jobs_owner_created_idx on jobs(owner_id, created_at desc);

create table if not exists job_steps (
  id uuid primary key default gen_random_uuid(),
  job_id uuid not null references jobs(id) on delete cascade,
  step step_name not null,
  status step_status not null default 'PENDING',
  attempt int not null default 0,
  last_error text null,
  updated_at timestamptz not null default now(),
  unique(job_id, step)
);
create index if not exists job_steps_job_idx on job_steps(job_id);

-- Optional outbox (useful for guaranteed publish; can be ignored initially)
create table if not exists outbox (
  id uuid primary key default gen_random_uuid(),
  aggregate_type text not null,
  aggregate_id uuid not null,
  event_type text not null,
  payload jsonb not null,
  created_at timestamptz not null default now(),
  published_at timestamptz null
);
create index if not exists outbox_published_idx on outbox(published_at);

-- updated_at helper trigger
create or replace function set_updated_at()
returns trigger language plpgsql as $$
begin
  new.updated_at = now();
  return new;
end $$;

drop trigger if exists trg_documents_updated_at on documents;
create trigger trg_documents_updated_at
before update on documents
for each row execute function set_updated_at();

drop trigger if exists trg_jobs_updated_at on jobs;
create trigger trg_jobs_updated_at
before update on jobs
for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists trg_jobs_updated_at on jobs;
drop trigger if exists trg_documents_updated_at on documents;
drop function if exists set_updated_at();

drop table if exists outbox;
drop table if exists job_steps;
drop table if exists jobs;
drop table if exists documents;
drop table if exists refresh_sessions;
drop table if exists users;

do $$ begin
  if exists (select 1 from pg_type where typname = 'step_status') then drop type step_status; end if;
  if exists (select 1 from pg_type where typname = 'step_name') then drop type step_name; end if;
  if exists (select 1 from pg_type where typname = 'job_status') then drop type job_status; end if;
  if exists (select 1 from pg_type where typname = 'doc_status') then drop type doc_status; end if;
end $$;
