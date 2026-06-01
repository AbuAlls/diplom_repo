package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// --- in-memory fakes (usecase-level) ---

type fakeDocRepo struct {
	mu     sync.Mutex
	byID   map[int64]domain.Document
	nextID int64
}

func newFakeDocRepo() *fakeDocRepo {
	return &fakeDocRepo{byID: map[int64]domain.Document{}}
}

func (r *fakeDocRepo) Create(_ context.Context, in ports.DocumentCreate) (domain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	now := time.Now()
	d := domain.Document{
		ID: r.nextID, PlanItemID: in.PlanItemID, Title: in.Title, FolderID: in.FolderID,
		UploadedBy: in.UploadedBy, Status: in.Status, FileName: in.FileName, FilePath: in.FilePath,
		MimeType: in.MimeType, FileSize: in.FileSize, CreatedAt: now, UpdatedAt: now,
	}
	r.byID[d.ID] = d
	return d, nil
}

func (r *fakeDocRepo) GetByID(_ context.Context, id int64) (domain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	return d, nil
}

func (r *fakeDocRepo) ListByOwner(context.Context, int64, *int64, int, int) ([]domain.Document, int, error) {
	return nil, 0, nil
}

func (r *fakeDocRepo) ListByOwners(context.Context, []int64, *int64, int, int) ([]domain.Document, int, error) {
	return nil, 0, nil
}

func (r *fakeDocRepo) UpdateStatusSerializable(_ context.Context, id int64, status string, expectFrom ...string) (domain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	if len(expectFrom) > 0 {
		matched := false
		for _, s := range expectFrom {
			if d.Status == s {
				matched = true
				break
			}
		}
		if !matched {
			return domain.Document{}, ports.ErrConflict
		}
	}
	d.Status = status
	d.UpdatedAt = time.Now()
	r.byID[id] = d
	return d, nil
}

func (r *fakeDocRepo) UpdateFields(_ context.Context, id int64, patch ports.DocumentPatch) (domain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	if patch.OrganizationName != nil {
		d.OrganizationName = patch.OrganizationName
	}
	if patch.ExternalNumber != nil {
		d.ExternalNumber = patch.ExternalNumber
	}
	if patch.INN != nil {
		d.INN = patch.INN
	}
	d.UpdatedAt = time.Now()
	r.byID[id] = d
	return d, nil
}

func (r *fakeDocRepo) UpdateStatus(_ context.Context, id int64, status string) (domain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	d.Status = status
	d.UpdatedAt = time.Now()
	r.byID[id] = d
	return d, nil
}

func (r *fakeDocRepo) CountByPlanItem(context.Context, int64) (int, error)          { return 0, nil }
func (r *fakeDocRepo) LatestDocIDByPlanItem(context.Context, int64) (*int64, error) { return nil, nil }

func (r *fakeDocRepo) status(id int64) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byID[id].Status
}

type fakeFolderRepo struct{}

func (fakeFolderRepo) FindOrCreateItemFolder(context.Context, int64, int64) (int64, error) {
	return 1, nil
}

type fakeFileStore struct {
	mu    sync.Mutex
	saved map[string][]byte
}

func newFakeFileStore() *fakeFileStore { return &fakeFileStore{saved: map[string][]byte{}} }

func (s *fakeFileStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.saved[key] = b
	s.mu.Unlock()
	return int64(len(b)), nil
}

func (s *fakeFileStore) Stat(_ context.Context, key string) (ports.FileStatus, error) {
	return ports.FileStatus{Key: key, Exists: true}, nil
}

func (s *fakeFileStore) Open(_ context.Context, key string) (ports.FileObject, error) {
	s.mu.Lock()
	b, ok := s.saved[key]
	s.mu.Unlock()
	if !ok {
		return ports.FileObject{}, ports.ErrNotFound
	}
	return ports.FileObject{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

type fakeExtractedRepo struct {
	mu    sync.Mutex
	byDoc map[int64]domain.ExtractedData
}

func newFakeExtractedRepo() *fakeExtractedRepo {
	return &fakeExtractedRepo{byDoc: map[int64]domain.ExtractedData{}}
}

func (r *fakeExtractedRepo) Create(_ context.Context, in ports.ExtractedDataCreate) (domain.ExtractedData, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := domain.ExtractedData{
		ID: in.DocumentID, DocumentID: in.DocumentID, RecognizedText: in.RecognizedText,
		StructuredJSON: in.StructuredJSON, ProcessingStatus: in.ProcessingStatus,
		ProcessedAt: in.ProcessedAt, ModelVersion: in.ModelVersion,
	}
	r.byDoc[in.DocumentID] = e
	return e, nil
}

func (r *fakeExtractedRepo) GetByDocumentID(_ context.Context, id int64) (domain.ExtractedData, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byDoc[id]
	if !ok {
		return domain.ExtractedData{}, ports.ErrNotFound
	}
	return e, nil
}

func (r *fakeExtractedRepo) ListByDocumentIDs(context.Context, []int64) (map[int64]domain.ExtractedData, error) {
	return nil, nil
}
func (r *fakeExtractedRepo) UpdateCategory(context.Context, int64, *int64) error { return nil }

func (r *fakeExtractedRepo) has(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byDoc[id]
	return ok
}

// fakeRecognizer returns a fixed result, or err for the first failUntil calls.
type fakeRecognizer struct {
	mu        sync.Mutex
	err       error
	failUntil int
	calls     int
}

func (f *fakeRecognizer) Recognize(_ context.Context, in ports.RecognizeInput) (ports.RecognizeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil && f.calls <= f.failUntil {
		return ports.RecognizeResult{}, f.err
	}
	model := "test-v0"
	org := "ACME"
	return ports.RecognizeResult{
		RecognizedText:   "recognized " + in.FileName,
		ModelVersion:     model,
		OrganizationName: &org,
	}, nil
}

func (f *fakeRecognizer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// memQueue is an in-memory ports.RecognitionQueue mirroring the Postgres
// adapter's semantics (Claim/Complete/Fail with attempts + backoff), so the
// worker can be tested without a database.
type memQueue struct {
	mu     sync.Mutex
	jobs   map[int64]*queueRow
	nextID int64
	now    time.Time
}

type queueRow struct {
	id          int64
	documentID  int64
	status      string // queued | running | done | failed
	attempts    int
	maxAttempts int
	runAfter    time.Time
	lastError   string
}

func newMemQueue() *memQueue {
	return &memQueue{jobs: map[int64]*queueRow{}, now: time.Now()}
}

// advance moves the queue's logical clock forward so jobs whose backoff has
// elapsed become claimable again. Tests use it to step through retries
// deterministically.
func (q *memQueue) advance(d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.now = q.now.Add(d)
}

func (q *memQueue) Enqueue(_ context.Context, documentID int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nextID++
	q.jobs[q.nextID] = &queueRow{
		id: q.nextID, documentID: documentID, status: "queued",
		maxAttempts: 3, runAfter: q.now,
	}
	return nil
}

func (q *memQueue) Claim(context.Context) (ports.RecognitionJob, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now
	// Pick the oldest runnable job (lowest id among queued & runAfter<=now).
	var pick *queueRow
	for _, r := range q.jobs {
		if r.status != "queued" || r.runAfter.After(now) {
			continue
		}
		if pick == nil || r.id < pick.id {
			pick = r
		}
	}
	if pick == nil {
		return ports.RecognitionJob{}, false, nil
	}
	pick.status = "running"
	return ports.RecognitionJob{
		ID: pick.id, DocumentID: pick.documentID,
		Attempts: pick.attempts, MaxAttempts: pick.maxAttempts,
	}, true, nil
}

func (q *memQueue) Complete(_ context.Context, jobID int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if r, ok := q.jobs[jobID]; ok {
		r.status = "done"
		r.lastError = ""
	}
	return nil
}

func (q *memQueue) Fail(_ context.Context, jobID int64, cause string, retryIn time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	r, ok := q.jobs[jobID]
	if !ok {
		return nil
	}
	r.attempts++
	r.lastError = cause
	r.runAfter = q.now.Add(retryIn)
	if r.attempts >= r.maxAttempts {
		r.status = "failed"
	} else {
		r.status = "queued"
	}
	return nil
}

func (q *memQueue) jobFor(documentID int64) *queueRow {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, r := range q.jobs {
		if r.documentID == documentID {
			cp := *r
			return &cp
		}
	}
	return nil
}

// --- test wiring ---

func asyncDocService(queue ports.RecognitionEnqueuer, rec ports.Recognizer) (*DocumentService, *fakeDocRepo, *fakeExtractedRepo) {
	docs := newFakeDocRepo()
	extracted := newFakeExtractedRepo()
	svc := &DocumentService{
		Docs:       docs,
		Folders:    fakeFolderRepo{},
		Items:      &stubItemRepo{getItem: domain.PlanItem{ID: 1, GoalID: 10}},
		Goals:      stubGoalRepo{goal: domain.Goal{ID: 10, PlanID: 5}},
		Plans:      stubPlanRepo{plan: domain.Plan{ID: 5, CreatedBy: 99}},
		Store:      newFakeFileStore(),
		Extracted:  extracted,
		Recognizer: rec,
		Queue:      queue,
	}
	return svc, docs, extracted
}

// --- tests: Option B (DB-backed queue + worker) ---

func TestUploadEnqueuesAndReturnsUploaded(t *testing.T) {
	queue := newMemQueue()
	svc, _, extracted := asyncDocService(queue, &fakeRecognizer{})

	view, err := svc.Upload(context.Background(), 99, 1, "contract.pdf", "application/pdf", strings.NewReader("bytes"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if view.Doc.Status != docStatusUploaded {
		t.Fatalf("expected status %q, got %q", docStatusUploaded, view.Doc.Status)
	}
	if view.Extracted != nil {
		t.Fatalf("expected no extracted data on async upload")
	}
	if extracted.has(view.Doc.ID) {
		t.Fatalf("recognition must not run inline on the upload path")
	}
	if job := queue.jobFor(view.Doc.ID); job == nil || job.status != "queued" {
		t.Fatalf("expected a queued job for doc %d, got %+v", view.Doc.ID, job)
	}
}

func TestWorkerProcessesJobToPendingReview(t *testing.T) {
	queue := newMemQueue()
	rec := &fakeRecognizer{}
	svc, docs, extracted := asyncDocService(queue, rec)

	view, err := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	docID := view.Doc.ID

	worker := &RecognitionWorker{Queue: queue, Processor: svc}
	// One claim+process cycle is enough; drain returns after the queue empties.
	worker.drain(context.Background())

	if got := docs.status(docID); got != docStatusPendingReview {
		t.Fatalf("expected %q after worker, got %q", docStatusPendingReview, got)
	}
	if !extracted.has(docID) {
		t.Fatalf("expected extracted data to be written by the worker")
	}
	if rec.callCount() != 1 {
		t.Fatalf("expected exactly one recognizer call, got %d", rec.callCount())
	}
	if job := queue.jobFor(docID); job == nil || job.status != "done" {
		t.Fatalf("expected job done, got %+v", job)
	}
}

func TestWorkerRetriesThenSucceeds(t *testing.T) {
	queue := newMemQueue()
	// Fail the first attempt, succeed on the second.
	rec := &fakeRecognizer{err: errors.New("ai timeout"), failUntil: 1}
	svc, docs, _ := asyncDocService(queue, rec)

	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID

	// Backoff of 1m: after a failure the job is not immediately runnable, so the
	// first drain processes exactly one (failing) attempt.
	worker := &RecognitionWorker{Queue: queue, Processor: svc, RetryBackoff: time.Minute}

	// First drain: the single attempt fails and is re-queued with backoff.
	worker.drain(context.Background())
	job := queue.jobFor(docID)
	if job == nil || job.attempts != 1 || job.status != "queued" {
		t.Fatalf("expected 1 attempt and re-queued after first failure, got %+v", job)
	}
	if got := docs.status(docID); got != docStatusFailed {
		// applyRecognition marks the doc failed on recognizer error.
		t.Fatalf("expected document failed after first failure, got %q", got)
	}

	// Backoff not elapsed yet → nothing to claim.
	worker.drain(context.Background())
	if got := queue.jobFor(docID); got.attempts != 1 {
		t.Fatalf("job must not be retried before backoff elapses, got %+v", got)
	}

	// Advance past the backoff window; second drain retries and succeeds.
	queue.advance(2 * time.Minute)
	worker.drain(context.Background())
	if got := docs.status(docID); got != docStatusPendingReview {
		t.Fatalf("expected pending_review after successful retry, got %q", got)
	}
	if job := queue.jobFor(docID); job == nil || job.status != "done" {
		t.Fatalf("expected job done after retry, got %+v", job)
	}
}

func TestWorkerGivesUpAfterMaxAttempts(t *testing.T) {
	queue := newMemQueue()
	rec := &fakeRecognizer{err: errors.New("permanent ai failure"), failUntil: 100}
	svc, docs, _ := asyncDocService(queue, rec)

	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID

	worker := &RecognitionWorker{Queue: queue, Processor: svc, RetryBackoff: time.Minute}
	// max_attempts is 3 → three attempts exhaust the retries, stepping the clock
	// past the backoff between each so the next attempt becomes runnable.
	for i := 0; i < 3; i++ {
		worker.drain(context.Background())
		queue.advance(2 * time.Minute)
	}

	job := queue.jobFor(docID)
	if job == nil || job.status != "failed" {
		t.Fatalf("expected job failed after max attempts, got %+v", job)
	}
	if job.attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", job.attempts)
	}
	if got := docs.status(docID); got != docStatusFailed {
		t.Fatalf("expected document failed, got %q", got)
	}
	if job.lastError == "" {
		t.Fatalf("expected last_error to be recorded")
	}

	// A further drain finds nothing runnable (job is terminal).
	worker.drain(context.Background())
	if rec.callCount() != 3 {
		t.Fatalf("expected exactly 3 recognizer calls, got %d", rec.callCount())
	}
}

func TestProcessRecognitionSkipsConfirmed(t *testing.T) {
	queue := newMemQueue()
	rec := &fakeRecognizer{}
	svc, docs, _ := asyncDocService(queue, rec)

	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID
	if _, err := docs.UpdateStatus(context.Background(), docID, docStatusConfirmed); err != nil {
		t.Fatalf("set confirmed: %v", err)
	}

	if err := svc.ProcessRecognition(context.Background(), docID); err != nil {
		t.Fatalf("ProcessRecognition: %v", err)
	}
	if rec.callCount() != 0 {
		t.Fatalf("confirmed document must not be recognized, got %d calls", rec.callCount())
	}
	if got := docs.status(docID); got != docStatusConfirmed {
		t.Fatalf("expected confirmed unchanged, got %q", got)
	}
}

// TestSerializableFinalizeDoesNotClobberConfirm models the async race the
// SERIALIZABLE conditional transition guards against: a corporate-account member
// confirms the document after recognition has produced data but before the
// worker writes its final pending_review status. The conditional finalize must
// detect that the document is no longer in a recognizable state and leave the
// member's `confirmed` decision intact.
func TestSerializableFinalizeDoesNotClobberConfirm(t *testing.T) {
	queue := newMemQueue()
	svc, docs, _ := asyncDocService(queue, &fakeRecognizer{})

	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID

	// Move to processing as the worker would.
	if _, err := docs.UpdateStatus(context.Background(), docID, docStatusProcessing); err != nil {
		t.Fatalf("set processing: %v", err)
	}
	// A member confirms concurrently (document now terminal).
	if _, err := docs.UpdateStatus(context.Background(), docID, docStatusConfirmed); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// applyRecognition's conditional finalize should NOT overwrite confirmed.
	doc, _, err := svc.applyRecognition(context.Background(), view.Doc, []byte("bytes"))
	if err != nil {
		t.Fatalf("applyRecognition: %v", err)
	}
	if doc.Status != docStatusConfirmed {
		t.Fatalf("member's confirm must survive the worker finalize, got %q", doc.Status)
	}
	if got := docs.status(docID); got != docStatusConfirmed {
		t.Fatalf("stored status must remain confirmed, got %q", got)
	}
}

// TestConfirmIsConditional verifies confirm only applies from pending_review,
// via the serializable conditional path.
func TestConfirmIsConditional(t *testing.T) {
	queue := newMemQueue()
	svc, docs, _ := asyncDocService(queue, &fakeRecognizer{})
	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID

	// Still `uploaded` (not pending_review) → confirm must conflict.
	if _, err := svc.Confirm(context.Background(), 99, docID); err != ErrConflict {
		t.Fatalf("expected ErrConflict confirming non-pending doc, got %v", err)
	}

	// Bring it to pending_review, then confirm succeeds.
	if _, err := docs.UpdateStatus(context.Background(), docID, docStatusPendingReview); err != nil {
		t.Fatalf("set pending_review: %v", err)
	}
	got, err := svc.Confirm(context.Background(), 99, docID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got.Doc.Status != docStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", got.Doc.Status)
	}
}

func TestWorkerRunStopsOnContextCancel(t *testing.T) {
	queue := newMemQueue()
	svc, docs, _ := asyncDocService(queue, &fakeRecognizer{})
	view, _ := svc.Upload(context.Background(), 99, 1, "doc.pdf", "application/pdf", strings.NewReader("bytes"))
	docID := view.Doc.ID

	worker := &RecognitionWorker{Queue: queue, Processor: svc, PollInterval: 5 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	// Wait until the job is processed, then cancel and ensure Run returns.
	deadline := time.After(2 * time.Second)
	for docs.status(docID) != docStatusPendingReview {
		select {
		case <-deadline:
			t.Fatalf("worker did not process job in time")
		case <-time.After(2 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("worker did not stop after context cancel")
	}
}
