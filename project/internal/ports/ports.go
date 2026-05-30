package ports

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"diplom.com/m/internal/domain"
)

// ErrNotFound is returned by repositories when a requested row does not exist.
var ErrNotFound = errors.New("not found")

type UserDTO struct {
	ID           int64
	Email        string
	FullName     string
	PasswordHash string
}

type UserRepo interface {
	Create(ctx context.Context, email, fullName, passwordHash string) (int64, error)
	GetByEmail(ctx context.Context, email string) (UserDTO, error)
	GetByID(ctx context.Context, id int64) (UserDTO, error)
}

type PlanRepo interface {
	Create(ctx context.Context, ownerID int64, name string, description *string, status string) (domain.Plan, error)
	GetByID(ctx context.Context, id int64) (domain.Plan, error)
	ListByOwner(ctx context.Context, ownerID int64, offset, limit int) (items []domain.Plan, total int, err error)
}

type GoalRepo interface {
	Create(ctx context.Context, planID int64, name string, description *string, sortOrder int) (domain.Goal, error)
	GetByID(ctx context.Context, id int64) (domain.Goal, error)
	ListByPlan(ctx context.Context, planID int64, offset, limit int) (items []domain.Goal, total int, err error)
}

type PlanItemInput struct {
	Name         string
	Description  *string
	SortOrder    int
	ItemType     string
	Status       string
	TargetValue  *float64
	CurrentValue *float64
	Unit         *string
}

type PlanItemPatch struct {
	Name         *string
	Description  *string
	SortOrder    *int
	ItemType     *string
	Status       *string
	TargetValue  *float64
	CurrentValue *float64
	Unit         *string
}

type PlanItemRepo interface {
	Create(ctx context.Context, goalID int64, in PlanItemInput) (domain.PlanItem, error)
	GetByID(ctx context.Context, id int64) (domain.PlanItem, error)
	ListByGoal(ctx context.Context, goalID int64, offset, limit int) (items []domain.PlanItem, total int, err error)
	Update(ctx context.Context, id int64, patch PlanItemPatch) (domain.PlanItem, error)
}

// FolderRepo resolves the system folder that holds a plan item's documents,
// creating it on first use (documents.folder_id is NOT NULL).
type FolderRepo interface {
	FindOrCreateItemFolder(ctx context.Context, planItemID, ownerID int64) (int64, error)
}

type DocumentCreate struct {
	PlanItemID int64
	FolderID   int64
	UploadedBy int64
	Title      string
	Status     string
	FileName   string
	FilePath   string
	MimeType   string
	FileSize   *int64
}

// DocumentPatch carries the manually-editable core `documents` columns. Each nil
// pointer leaves the column unchanged; slices are applied when non-nil.
type DocumentPatch struct {
	DocumentDate     *time.Time
	ExternalNumber   *string
	OrganizationName *string
	INN              *string
	Description      *string
	Deadlines        []time.Time
	PersonalData     []string
	OrganizationData []string
	Prices           []string
	Quantities       []int32
	ProductNames     []string
	ContractNumbers  []string
}

type DocumentRepo interface {
	Create(ctx context.Context, in DocumentCreate) (domain.Document, error)
	GetByID(ctx context.Context, id int64) (domain.Document, error)
	ListByOwner(ctx context.Context, ownerID int64, planItemID *int64, offset, limit int) (items []domain.Document, total int, err error)
	UpdateFields(ctx context.Context, id int64, patch DocumentPatch) (domain.Document, error)
	UpdateStatus(ctx context.Context, id int64, status string) (domain.Document, error)
	CountByPlanItem(ctx context.Context, planItemID int64) (int, error)
	LatestDocIDByPlanItem(ctx context.Context, planItemID int64) (*int64, error)
}

type ExtractedDataCreate struct {
	DocumentID           int64
	RecognizedText       string
	StructuredJSON       json.RawMessage
	RecognizedCategoryID *int64
	ConfidenceScore      *float64
	ProcessingStatus     string
	ProcessedAt          *time.Time
	ModelVersion         *string
}

type ExtractedDataRepo interface {
	Create(ctx context.Context, in ExtractedDataCreate) (domain.ExtractedData, error)
	GetByDocumentID(ctx context.Context, documentID int64) (domain.ExtractedData, error)
	ListByDocumentIDs(ctx context.Context, documentIDs []int64) (map[int64]domain.ExtractedData, error)
	UpdateCategory(ctx context.Context, documentID int64, categoryID *int64) error
}

// FileStore persists uploaded bytes and returns the number of bytes written.
type FileStatus struct {
	Key           string
	Exists        bool
	StatusCode    int
	ContentLength *int64
	ContentType   string
	ETag          string
	LastModified  *time.Time
}

type FileObject struct {
	Status FileStatus
	Body   io.ReadCloser
}

type FileStore interface {
	Save(ctx context.Context, key string, r io.Reader) (size int64, err error)
	Stat(ctx context.Context, key string) (FileStatus, error)
	Open(ctx context.Context, key string) (FileObject, error)
}

type RecognizeInput struct {
	DocumentID int64
	FileName   string
	MimeType   string
}

// RecognizeResult is the output of the (mock) recognition service: the analysis
// payload plus the structured document fields it inferred.
type RecognizeResult struct {
	RecognizedText       string
	StructuredJSON       json.RawMessage
	RecognizedCategoryID *int64
	ConfidenceScore      *float64
	ModelVersion         string

	DocumentDate     *time.Time
	ExternalNumber   *string
	OrganizationName *string
	INN              *string
	Deadlines        []time.Time
	PersonalData     []string
	OrganizationData []string
	Prices           []string
	Quantities       []int32
	ProductNames     []string
	ContractNumbers  []string
}

// Recognizer is the OCR/LLM analysis port. The v0 implementation is a deterministic mock.
type Recognizer interface {
	Recognize(ctx context.Context, in RecognizeInput) (RecognizeResult, error)
}
