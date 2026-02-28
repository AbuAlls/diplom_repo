package pgcore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

type DocumentRepo struct{ Store *Store }

type JobRepo struct{ Store *Store }

type UserRepo struct{ Store *Store }

type SessionRepo struct{ Store *Store }

func NewDocumentRepo(store *Store) *DocumentRepo { return &DocumentRepo{Store: store} }
func NewJobRepo(store *Store) *JobRepo           { return &JobRepo{Store: store} }
func NewUserRepo(store *Store) *UserRepo         { return &UserRepo{Store: store} }
func NewSessionRepo(store *Store) *SessionRepo   { return &SessionRepo{Store: store} }

func (r *DocumentRepo) Create(ctx context.Context, ownerID, filename, mime, checksum, objectKey string, size int64) (string, error) {
	const q = `
insert into documents (owner_id, filename, mime, size, checksum, object_key)
values ($1, $2, $3, $4, $5, $6)
returning id::text`
	var id string
	if err := r.Store.Pool.QueryRow(ctx, q, ownerID, filename, mime, size, checksum, objectKey).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *DocumentRepo) Get(ctx context.Context, ownerID, docID string) (ports.DocumentDTO, error) {
	const q = `
select id::text, owner_id::text, filename, mime, size, checksum, object_key, status::text, created_at, updated_at
from documents
where id = $1 and owner_id = $2`
	var d ports.DocumentDTO
	var status string
	err := r.Store.Pool.QueryRow(ctx, q, docID, ownerID).Scan(
		&d.ID, &d.OwnerID, &d.Filename, &d.Mime, &d.Size, &d.Checksum, &d.ObjectKey, &status, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return ports.DocumentDTO{}, err
	}
	d.Status = domain.DocStatus(status)
	return d, nil
}

func (r *DocumentRepo) List(ctx context.Context, ownerID string, limit int, cursor string, status *domain.DocStatus) ([]ports.DocumentDTO, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	args := []any{ownerID}
	where := "where owner_id = $1"
	if status != nil {
		args = append(args, string(*status))
		where += fmt.Sprintf(" and status = $%d::doc_status", len(args))
	}
	if cursor != "" {
		args = append(args, cursor)
		where += fmt.Sprintf(" and created_at < $%d", len(args))
	}
	args = append(args, limit+1)
	q := fmt.Sprintf(`
select id::text, owner_id::text, filename, mime, size, checksum, object_key, status::text, created_at, updated_at
from documents
%s
order by created_at desc
limit $%d`, where, len(args))
	rows, err := r.Store.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]ports.DocumentDTO, 0, limit)
	var next string
	for rows.Next() {
		var d ports.DocumentDTO
		var st string
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Filename, &d.Mime, &d.Size, &d.Checksum, &d.ObjectKey, &st, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, "", err
		}
		d.Status = domain.DocStatus(st)
		out = append(out, d)
	}
	if len(out) > limit {
		next = out[limit-1].CreatedAt.Format(time.RFC3339Nano)
		out = out[:limit]
	}
	return out, next, rows.Err()
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, docID string, status domain.DocStatus) error {
	_, err := r.Store.Pool.Exec(ctx, `update documents set status = $2 where id = $1`, docID, string(status))
	return err
}

func (r *JobRepo) Create(ctx context.Context, ownerID, docID string, pipelineVersion int) (string, error) {
	const q = `insert into jobs (document_id, owner_id, pipeline_version) values ($1, $2, $3) returning id::text`
	var id string
	if err := r.Store.Pool.QueryRow(ctx, q, docID, ownerID, pipelineVersion).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *JobRepo) GetByOwner(ctx context.Context, ownerID, jobID string) (ports.JobDTO, error) {
	const q = `select id::text, document_id::text, owner_id::text, status::text, created_at from jobs where id = $1 and owner_id = $2`
	var out ports.JobDTO
	err := r.Store.Pool.QueryRow(ctx, q, jobID, ownerID).Scan(&out.ID, &out.DocumentID, &out.OwnerID, &out.Status, &out.CreatedAt)
	return out, err
}

func (r *JobRepo) GetByID(ctx context.Context, jobID string) (ports.JobDTO, error) {
	const q = `select id::text, document_id::text, owner_id::text, status::text, created_at from jobs where id = $1`
	var out ports.JobDTO
	err := r.Store.Pool.QueryRow(ctx, q, jobID).Scan(&out.ID, &out.DocumentID, &out.OwnerID, &out.Status, &out.CreatedAt)
	return out, err
}

func (r *JobRepo) UpsertStep(ctx context.Context, jobID string, step domain.StepName) error {
	_, err := r.Store.Pool.Exec(ctx, `insert into job_steps (job_id, step) values ($1, $2) on conflict (job_id, step) do nothing`, jobID, string(step))
	return err
}

func (r *JobRepo) MarkStepRunning(ctx context.Context, jobID string, step domain.StepName) (int, error) {
	const q = `update job_steps set status = 'RUNNING', attempt = attempt + 1, updated_at = now() where job_id = $1 and step = $2 returning attempt`
	var attempt int
	if err := r.Store.Pool.QueryRow(ctx, q, jobID, string(step)).Scan(&attempt); err != nil {
		return 0, err
	}
	return attempt, nil
}

func (r *JobRepo) MarkStepCompleted(ctx context.Context, jobID string, step domain.StepName) error {
	_, err := r.Store.Pool.Exec(ctx, `update job_steps set status = 'COMPLETED', updated_at = now() where job_id = $1 and step = $2`, jobID, string(step))
	return err
}

func (r *JobRepo) MarkStepFailed(ctx context.Context, jobID string, step domain.StepName, errMsg string) (int, error) {
	const q = `update job_steps set status = 'FAILED', last_error = $3, attempt = attempt + 1, updated_at = now() where job_id = $1 and step = $2 returning attempt`
	var attempt int
	if err := r.Store.Pool.QueryRow(ctx, q, jobID, string(step), errMsg).Scan(&attempt); err != nil {
		return 0, err
	}
	return attempt, nil
}

func (r *JobRepo) MarkJobRunning(ctx context.Context, jobID string) error {
	_, err := r.Store.Pool.Exec(ctx, `update jobs set status = 'RUNNING' where id = $1`, jobID)
	return err
}

func (r *JobRepo) MarkJobCompleted(ctx context.Context, jobID string) error {
	_, err := r.Store.Pool.Exec(ctx, `update jobs set status = 'COMPLETED' where id = $1`, jobID)
	return err
}

func (r *JobRepo) MarkJobFailed(ctx context.Context, jobID string) error {
	_, err := r.Store.Pool.Exec(ctx, `update jobs set status = 'FAILED' where id = $1`, jobID)
	return err
}

func (r *JobRepo) ListCreatedJobs(ctx context.Context, limit int) ([]ports.JobDTO, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.Store.Pool.Query(ctx, `
select id::text, document_id::text, owner_id::text, status::text, created_at
from jobs where status = 'CREATED' order by created_at asc limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.JobDTO, 0, limit)
	for rows.Next() {
		var j ports.JobDTO
		if err := rows.Scan(&j.ID, &j.DocumentID, &j.OwnerID, &j.Status, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *UserRepo) Create(ctx context.Context, email, passwordHash string) (string, error) {
	var id string
	if err := r.Store.Pool.QueryRow(ctx, `insert into users (email, password_hash) values ($1, $2) returning id::text`, email, passwordHash).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (ports.UserDTO, error) {
	var u ports.UserDTO
	err := r.Store.Pool.QueryRow(ctx, `select id::text, email, password_hash, created_at from users where email = $1`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) GetByID(ctx context.Context, userID string) (ports.UserDTO, error) {
	var u ports.UserDTO
	err := r.Store.Pool.QueryRow(ctx, `select id::text, email, password_hash, created_at from users where id = $1`, userID).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (r *SessionRepo) Create(ctx context.Context, userID, refreshHash string, expiresAt time.Time, userAgent, ip string) (string, error) {
	const q = `
insert into refresh_sessions (user_id, refresh_hash, expires_at, user_agent, ip)
values ($1, $2, $3, $4, nullif($5,'')::inet)
returning id::text`
	var id string
	if err := r.Store.Pool.QueryRow(ctx, q, userID, refreshHash, expiresAt, userAgent, ip).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *SessionRepo) GetValid(ctx context.Context, sessionID string) (ports.RefreshSessionDTO, error) {
	const q = `select id::text, user_id::text, refresh_hash, expires_at, revoked_at, coalesce(user_agent,''), coalesce(host(ip),'') from refresh_sessions where id = $1`
	var out ports.RefreshSessionDTO
	err := r.Store.Pool.QueryRow(ctx, q, sessionID).Scan(&out.ID, &out.UserID, &out.RefreshHash, &out.ExpiresAt, &out.RevokedAt, &out.UserAgent, &out.IP)
	if err != nil {
		return ports.RefreshSessionDTO{}, err
	}
	if out.RevokedAt != nil {
		return ports.RefreshSessionDTO{}, errors.New("revoked")
	}
	return out, nil
}

func (r *SessionRepo) Revoke(ctx context.Context, sessionID string) error {
	_, err := r.Store.Pool.Exec(ctx, `update refresh_sessions set revoked_at = now() where id = $1 and revoked_at is null`, sessionID)
	return err
}

var _ ports.DocumentRepo = (*DocumentRepo)(nil)
var _ ports.JobRepo = (*JobRepo)(nil)
var _ ports.UserRepo = (*UserRepo)(nil)
var _ ports.SessionRepo = (*SessionRepo)(nil)
