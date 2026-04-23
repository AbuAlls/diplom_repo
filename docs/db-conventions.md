# DB conventions

- primary keys: BIGSERIAL
- foreign keys: BIGINT
- timestamps: TIMESTAMP NOT NULL DEFAULT NOW()
- status fields stored as VARCHAR, mapped to enums in Go
- arrays allowed only for extracted/denormalized document metadata
- analytics tables are append-oriented or projection-oriented