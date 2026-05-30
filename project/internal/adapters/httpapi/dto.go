package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/usecase"
)

const dateLayout = "2006-01-02"

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type planResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatedBy   int64   `json:"created_by"`
	Status      string  `json:"status"`
}

type planDetailResponse struct {
	planResponse
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type goalResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   int     `json:"sort_order"`
}

type planItemResponse struct {
	ID              int64    `json:"id"`
	GoalID          int64    `json:"goal_id"`
	GoalName        string   `json:"goal_name,omitempty"`
	Name            string   `json:"name"`
	Description     *string  `json:"description"`
	SortOrder       int      `json:"sort_order"`
	ItemType        *string  `json:"item_type"`
	Status          *string  `json:"status"`
	TargetValue     *float64 `json:"target_value"`
	CurrentValue    *float64 `json:"current_value"`
	Unit            *string  `json:"unit"`
	ProgressPercent *float64 `json:"progress_percent"`
}

type listResponse struct {
	Items any            `json:"items"`
	Meta  PaginationMeta `json:"meta"`
}

func toPlan(p domain.Plan) planResponse {
	return planResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedBy:   p.CreatedBy,
		Status:      p.Status,
	}
}

func toPlanDetail(p domain.Plan) planDetailResponse {
	return planDetailResponse{
		planResponse: toPlan(p),
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func toGoal(g domain.Goal) goalResponse {
	return goalResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		SortOrder:   g.SortOrder,
	}
}

type documentResponse struct {
	ID                   int64           `json:"id"`
	PlanItemID           int64           `json:"plan_item_id"`
	Title                string          `json:"title"`
	RecognizedText       string          `json:"recognized_text"`
	StructuredJSON       json.RawMessage `json:"structured_json"`
	RecognizedCategoryID *int64          `json:"recognized_category_id"`
	ConfidenceScore      *float64        `json:"confidence_score"`
	ModelVersion         *string         `json:"model_version"`
	DocumentDate         *string         `json:"document_date"`
	ExternalNumber       *string         `json:"external_number"`
	OrganizationName     *string         `json:"organization_name"`
	INN                  *string         `json:"inn"`
	Description          *string         `json:"description"`
	FileName             string          `json:"file_name"`
	FilePath             string          `json:"file_path"`
	MimeType             string          `json:"mime_type"`
	FileSize             *int64          `json:"file_size"`
	Deadlines            []string        `json:"deadlines"`
	PersonalData         []string        `json:"personal_data"`
	OrganizationData     []string        `json:"organization_data"`
	Prices               []string        `json:"prices"`
	Quantities           []int32         `json:"quantities"`
	ProductNames         []string        `json:"product_names"`
	ContractNumbers      []string        `json:"contract_numbers"`
	Status               string          `json:"status"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type documentStorageResponse struct {
	DocumentID       int64      `json:"document_id"`
	FileName         string     `json:"file_name"`
	FilePath         string     `json:"file_path"`
	MimeType         string     `json:"mime_type"`
	DbFileSize       *int64     `json:"db_file_size"`
	ObjectExists     bool       `json:"object_exists"`
	ObjectStatus     string     `json:"object_status"`
	ObjectStatusCode int        `json:"object_status_code"`
	ObjectSize       *int64     `json:"object_size"`
	ObjectType       string     `json:"object_type,omitempty"`
	ObjectETag       string     `json:"object_etag,omitempty"`
	LastModified     *time.Time `json:"last_modified,omitempty"`
	CheckedAt        time.Time  `json:"checked_at"`
}

func toDocument(v usecase.DocumentView) documentResponse {
	d := v.Doc
	resp := documentResponse{
		ID:               d.ID,
		PlanItemID:       d.PlanItemID,
		Title:            d.Title,
		StructuredJSON:   json.RawMessage("{}"),
		ExternalNumber:   d.ExternalNumber,
		OrganizationName: d.OrganizationName,
		INN:              d.INN,
		Description:      d.Description,
		FileName:         d.FileName,
		FilePath:         d.FilePath,
		MimeType:         d.MimeType,
		FileSize:         d.FileSize,
		PersonalData:     d.PersonalData,
		OrganizationData: d.OrganizationData,
		Prices:           d.Prices,
		Quantities:       d.Quantities,
		ProductNames:     d.ProductNames,
		ContractNumbers:  d.ContractNumbers,
		Status:           d.Status,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
	if d.DocumentDate != nil {
		s := d.DocumentDate.Format(dateLayout)
		resp.DocumentDate = &s
	}
	for _, t := range d.Deadlines {
		resp.Deadlines = append(resp.Deadlines, t.Format(dateLayout))
	}
	if e := v.Extracted; e != nil {
		resp.RecognizedText = e.RecognizedText
		if len(e.StructuredJSON) > 0 {
			resp.StructuredJSON = e.StructuredJSON
		}
		resp.RecognizedCategoryID = e.RecognizedCategoryID
		resp.ConfidenceScore = e.ConfidenceScore
		resp.ModelVersion = e.ModelVersion
	}
	return resp
}

func toDocumentStorage(s usecase.DocumentStorageStatus) documentStorageResponse {
	objectStatus := "missing"
	if s.Object.Exists {
		objectStatus = "available"
	} else if s.Object.StatusCode > 0 && s.Object.StatusCode != http.StatusNotFound {
		objectStatus = "unavailable"
	}
	return documentStorageResponse{
		DocumentID:       s.Doc.ID,
		FileName:         s.Doc.FileName,
		FilePath:         s.Doc.FilePath,
		MimeType:         s.Doc.MimeType,
		DbFileSize:       s.Doc.FileSize,
		ObjectExists:     s.Object.Exists,
		ObjectStatus:     objectStatus,
		ObjectStatusCode: s.Object.StatusCode,
		ObjectSize:       s.Object.ContentLength,
		ObjectType:       s.Object.ContentType,
		ObjectETag:       s.Object.ETag,
		LastModified:     s.Object.LastModified,
		CheckedAt:        s.CheckedAt,
	}
}

type itemAnalyticsResponse struct {
	Item                 planItemResponse `json:"item"`
	LatestDocumentID     *int64           `json:"latest_document_id"`
	SourceDocumentsCount int              `json:"source_documents_count"`
	CurrentValue         *float64         `json:"current_value"`
	TargetValue          *float64         `json:"target_value"`
	ProgressPercent      *float64         `json:"progress_percent"`
	Unit                 *string          `json:"unit"`
	UpdatedAt            time.Time        `json:"updated_at"`
	Notes                []string         `json:"notes"`
}

func toItemAnalytics(a usecase.ItemAnalytics) itemAnalyticsResponse {
	notes := a.Notes
	if notes == nil {
		notes = []string{}
	}
	return itemAnalyticsResponse{
		Item:                 toPlanItem(a.Item),
		LatestDocumentID:     a.LatestDocumentID,
		SourceDocumentsCount: a.SourceDocumentsCount,
		CurrentValue:         a.Item.CurrentValue,
		TargetValue:          a.Item.TargetValue,
		ProgressPercent:      a.Item.ProgressPercent(),
		Unit:                 a.Item.Unit,
		UpdatedAt:            a.Item.UpdatedAt,
		Notes:                notes,
	}
}

func toPlanItem(i domain.PlanItem) planItemResponse {
	return planItemResponse{
		ID:              i.ID,
		GoalID:          i.GoalID,
		GoalName:        i.GoalName,
		Name:            i.Name,
		Description:     i.Description,
		SortOrder:       i.SortOrder,
		ItemType:        i.ItemType,
		Status:          i.Status,
		TargetValue:     i.TargetValue,
		CurrentValue:    i.CurrentValue,
		Unit:            i.Unit,
		ProgressPercent: i.ProgressPercent(),
	}
}
