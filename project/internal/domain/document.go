package domain

import (
	"encoding/json"
	"time"
)

// Document is the core `documents` row plus the derived plan_item_id (resolved
// through its folder).
type Document struct {
	ID         int64
	PlanItemID int64
	Title      string
	FolderID   int64
	CategoryID *int64
	UploadedBy int64
	Status     string

	DocumentDate     *time.Time
	ExternalNumber   *string
	OrganizationName *string
	INN              *string
	Description      *string

	FileName string
	FilePath string
	MimeType string
	FileSize *int64

	Deadlines        []time.Time
	PersonalData     []string
	OrganizationData []string
	Prices           []string
	Quantities       []int32
	ProductNames     []string
	ContractNumbers  []string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ExtractedData is the analysis-side `extracted_document_data` row produced by
// recognition for a document.
type ExtractedData struct {
	ID                   int64
	DocumentID           int64
	RecognizedText       string
	StructuredJSON       json.RawMessage
	RecognizedCategoryID *int64
	ConfidenceScore      *float64
	ProcessingStatus     string
	ProcessedAt          *time.Time
	ModelVersion         *string
}
