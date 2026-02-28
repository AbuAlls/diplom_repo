-- +goose Up
create extension if not exists pgcrypto;

create table if not exists extractions (
  id uuid primary key default gen_random_uuid(),
  document_id uuid not null,
  owner_id uuid not null,
  schema_version int not null default 1,
  created_at timestamptz not null default now()
);
create index if not exists extractions_owner_created_idx on extractions(owner_id, created_at desc);
create index if not exists extractions_document_idx on extractions(document_id);

create table if not exists extracted_fields (
  extraction_id uuid not null references extractions(id) on delete cascade,
  key text not null,
  value text not null,
  confidence real null,
  meta jsonb null,
  primary key (extraction_id, key)
);

-- +goose Down
drop table if exists extracted_fields;
drop table if exists extractions;
