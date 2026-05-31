package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

const (
	docStatusProcessing    = "processing"
	docStatusPendingReview = "pending_review"
	docStatusConfirmed     = "confirmed"
	docStatusFailed        = "failed"
)

type DocumentService struct {
	Docs       ports.DocumentRepo
	Folders    ports.FolderRepo
	Items      ports.PlanItemRepo
	Goals      ports.GoalRepo
	Plans      ports.PlanRepo
	Store      ports.FileStore
	Extracted  ports.ExtractedDataRepo
	Recognizer ports.Recognizer
}

// DocumentView is the merged read model: the core document plus its analysis-side
// extracted data (nil until recognition has run).
type DocumentView struct {
	Doc       domain.Document
	Extracted *domain.ExtractedData
}

// DocumentUpdate carries an edit of recognized fields. SetCategory distinguishes
// "set recognized_category_id (possibly to null)" from "field absent".
type DocumentUpdate struct {
	SetCategory          bool
	RecognizedCategoryID *int64
	Fields               ports.DocumentPatch
}

type DocumentStorageStatus struct {
	Doc       domain.Document
	Object    ports.FileStatus
	CheckedAt time.Time
}

type DocumentDownload struct {
	Doc    domain.Document
	Object ports.FileObject
}

func (s *DocumentService) Upload(ctx context.Context, ownerID, planItemID int64, fileName, mimeType string, file io.Reader) (DocumentView, error) {
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, planItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return DocumentView{}, ErrValidation
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	folderID, err := s.Folders.FindOrCreateItemFolder(ctx, planItemID, ownerID)
	if err != nil {
		return DocumentView{}, err
	}

	// Buffer the upload once so we can both persist it and hand the bytes to
	// the recognizer (the real AI client posts them; the mock ignores them).
	content, err := io.ReadAll(file)
	if err != nil {
		return DocumentView{}, err
	}

	key := fmt.Sprintf("plan_item_%d/%d_%s", planItemID, time.Now().UnixNano(), sanitizeFileName(fileName))
	size, err := s.Store.Save(ctx, key, bytes.NewReader(content))
	if err != nil {
		return DocumentView{}, err
	}

	doc, err := s.Docs.Create(ctx, ports.DocumentCreate{
		PlanItemID: planItemID,
		FolderID:   folderID,
		UploadedBy: ownerID,
		Title:      fileName,
		Status:     docStatusProcessing,
		FileName:   fileName,
		FilePath:   key,
		MimeType:   mimeType,
		FileSize:   &size,
	})
	if err != nil {
		return DocumentView{}, err
	}

	res, err := s.Recognizer.Recognize(ctx, ports.RecognizeInput{DocumentID: doc.ID, FileName: fileName, MimeType: mimeType, Content: content})
	if err != nil {
		_, _ = s.Docs.UpdateStatus(ctx, doc.ID, docStatusFailed)
		return DocumentView{}, err
	}

	now := time.Now()
	modelVersion := res.ModelVersion
	extracted, err := s.Extracted.Create(ctx, ports.ExtractedDataCreate{
		DocumentID:           doc.ID,
		RecognizedText:       res.RecognizedText,
		StructuredJSON:       res.StructuredJSON,
		RecognizedCategoryID: res.RecognizedCategoryID,
		ConfidenceScore:      res.ConfidenceScore,
		ProcessingStatus:     "processed",
		ProcessedAt:          &now,
		ModelVersion:         &modelVersion,
	})
	if err != nil {
		return DocumentView{}, err
	}

	if _, err := s.Docs.UpdateFields(ctx, doc.ID, ports.DocumentPatch{
		DocumentDate:     res.DocumentDate,
		ExternalNumber:   res.ExternalNumber,
		OrganizationName: res.OrganizationName,
		INN:              res.INN,
		Deadlines:        res.Deadlines,
		PersonalData:     res.PersonalData,
		OrganizationData: res.OrganizationData,
		Prices:           res.Prices,
		Quantities:       res.Quantities,
		ProductNames:     res.ProductNames,
		ContractNumbers:  res.ContractNumbers,
	}); err != nil {
		return DocumentView{}, err
	}
	doc, err = s.Docs.UpdateStatus(ctx, doc.ID, docStatusPendingReview)
	if err != nil {
		return DocumentView{}, err
	}
	return DocumentView{Doc: doc, Extracted: &extracted}, nil
}

func (s *DocumentService) List(ctx context.Context, ownerID int64, planItemID *int64, page, size int) ([]DocumentView, int, error) {
	if planItemID != nil {
		if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, *planItemID, ownerID); err != nil {
			return nil, 0, err
		}
	}
	offset, limit := offsetLimit(page, size)
	docs, total, err := s.Docs.ListByOwner(ctx, ownerID, planItemID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.attachExtracted(ctx, docs)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *DocumentService) Get(ctx context.Context, ownerID, docID int64) (DocumentView, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentView{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) StorageStatus(ctx context.Context, ownerID, docID int64) (DocumentStorageStatus, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentStorageStatus{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, doc.PlanItemID, ownerID); err != nil {
		return DocumentStorageStatus{}, err
	}
	status, err := s.Store.Stat(ctx, doc.FilePath)
	if err != nil {
		return DocumentStorageStatus{}, err
	}
	return DocumentStorageStatus{
		Doc:       doc,
		Object:    status,
		CheckedAt: time.Now(),
	}, nil
}

func (s *DocumentService) Download(ctx context.Context, ownerID, docID int64) (DocumentDownload, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentDownload{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, doc.PlanItemID, ownerID); err != nil {
		return DocumentDownload{}, err
	}
	obj, err := s.Store.Open(ctx, doc.FilePath)
	if err != nil {
		return DocumentDownload{}, err
	}
	return DocumentDownload{Doc: doc, Object: obj}, nil
}

func (s *DocumentService) Patch(ctx context.Context, ownerID, docID int64, upd DocumentUpdate) (DocumentView, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentView{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	if upd.SetCategory {
		if err := s.Extracted.UpdateCategory(ctx, docID, upd.RecognizedCategoryID); err != nil {
			return DocumentView{}, err
		}
	}
	doc, err = s.Docs.UpdateFields(ctx, docID, upd.Fields)
	if err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) Confirm(ctx context.Context, ownerID, docID int64) (DocumentView, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentView{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	if doc.Status != docStatusPendingReview {
		return DocumentView{}, ErrConflict
	}
	doc, err = s.Docs.UpdateStatus(ctx, docID, docStatusConfirmed)
	if err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) viewOf(ctx context.Context, doc domain.Document) (DocumentView, error) {
	e, err := s.Extracted.GetByDocumentID(ctx, doc.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return DocumentView{Doc: doc}, nil
		}
		return DocumentView{}, err
	}
	return DocumentView{Doc: doc, Extracted: &e}, nil
}

func (s *DocumentService) attachExtracted(ctx context.Context, docs []domain.Document) ([]DocumentView, error) {
	ids := make([]int64, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	byDoc, err := s.Extracted.ListByDocumentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]DocumentView, len(docs))
	for i, d := range docs {
		v := DocumentView{Doc: d}
		if e, ok := byDoc[d.ID]; ok {
			ec := e
			v.Extracted = &ec
		}
		out[i] = v
	}
	return out, nil
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.ReplaceAll(name, " ", "_")
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}
