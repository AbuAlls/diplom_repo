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
	docStatusUploaded      = "uploaded"
	docStatusProcessing    = "processing"
	docStatusPendingReview = "pending_review"
	docStatusConfirmed     = "confirmed"
	docStatusFailed        = "failed"
	docStatusRejected      = "rejected"
)

type DocumentService struct {
	Docs       ports.DocumentRepo
	Folders    ports.FolderRepo
	Items      ports.PlanItemRepo
	Goals      ports.GoalRepo
	Plans      ports.PlanRepo
	Groups     ports.GroupRepo
	Store      ports.FileStore
	Extracted  ports.ExtractedDataRepo
	Recognizer ports.Recognizer

	// Queue hands uploaded documents off for asynchronous recognition. When
	// nil, Upload falls back to running recognition inline (synchronous), which
	// keeps older wiring and tests working without a worker.
	Queue ports.RecognitionEnqueuer
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
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, planItemID, ownerID); err != nil {
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

	// Async path: persist the row as `uploaded` and hand recognition off to the
	// queue, so the upload request returns immediately. The recognizer reads the
	// bytes back from object storage when the worker picks the job up.
	if s.Queue != nil {
		doc, err := s.Docs.Create(ctx, ports.DocumentCreate{
			PlanItemID: planItemID,
			FolderID:   folderID,
			UploadedBy: ownerID,
			Title:      fileName,
			Status:     docStatusUploaded,
			FileName:   fileName,
			FilePath:   key,
			MimeType:   mimeType,
			FileSize:   &size,
		})
		if err != nil {
			return DocumentView{}, err
		}
		if err := s.Queue.Enqueue(ctx, doc.ID); err != nil {
			return DocumentView{}, err
		}
		return DocumentView{Doc: doc}, nil
	}

	// Synchronous fallback (no queue wired): recognize inline before returning.
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

	doc, extracted, err := s.applyRecognition(ctx, doc, content)
	if err != nil {
		return DocumentView{}, err
	}
	return DocumentView{Doc: doc, Extracted: &extracted}, nil
}

// ProcessRecognition runs recognition for one document end to end: it loads the
// row, reads the file back from object storage, calls the recognizer, and writes
// the extracted data (status → pending_review) or marks the document failed.
// This is the worker-facing entry point; it carries no ownership check because
// the background worker acts on behalf of the system, not a specific caller.
func (s *DocumentService) ProcessRecognition(ctx context.Context, documentID int64) error {
	doc, err := s.Docs.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	// Confirmed documents are terminal; never re-process them.
	if doc.Status == docStatusConfirmed {
		return nil
	}

	obj, err := s.Store.Open(ctx, doc.FilePath)
	if err != nil {
		_, _ = s.Docs.UpdateStatus(ctx, doc.ID, docStatusFailed)
		return err
	}
	defer obj.Body.Close()
	content, err := io.ReadAll(obj.Body)
	if err != nil {
		_, _ = s.Docs.UpdateStatus(ctx, doc.ID, docStatusFailed)
		return err
	}

	// SERIALIZABLE transition into `processing`, conditional on the document not
	// already being terminal/in-flight. Because corporate-account members share
	// this document, another member (or a duplicate job) could be acting on it
	// concurrently; the serializable, conditional claim ensures exactly one
	// worker advances it from a non-confirmed state. ErrConflict here means
	// someone else already moved it — treat that as a no-op success.
	doc, err = s.Docs.UpdateStatusSerializable(ctx, doc.ID, docStatusProcessing,
		docStatusUploaded, docStatusFailed, docStatusProcessing, docStatusPendingReview, docStatusRejected)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return nil
		}
		return err
	}
	if _, _, err := s.applyRecognition(ctx, doc, content); err != nil {
		return err
	}
	return nil
}

func (s *DocumentService) applyRecognition(ctx context.Context, doc domain.Document, content []byte) (domain.Document, domain.ExtractedData, error) {
	res, err := s.Recognizer.Recognize(ctx, ports.RecognizeInput{DocumentID: doc.ID, FileName: doc.FileName, MimeType: doc.MimeType, Content: content})
	if err != nil {
		_, _ = s.Docs.UpdateStatus(ctx, doc.ID, docStatusFailed)
		return domain.Document{}, domain.ExtractedData{}, err
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
		return domain.Document{}, domain.ExtractedData{}, err
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
		return domain.Document{}, domain.ExtractedData{}, err
	}
	// Finalize to pending_review under SERIALIZABLE so a concurrent confirm/
	// reject by a corporate-account member can't be silently overwritten: if the
	// document is no longer in a recognizable state, keep the member's decision.
	finalized, err := s.Docs.UpdateStatusSerializable(ctx, doc.ID, docStatusPendingReview,
		docStatusProcessing, docStatusUploaded, docStatusFailed)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			// Someone already confirmed/rejected; return the current row as-is.
			cur, gerr := s.Docs.GetByID(ctx, doc.ID)
			if gerr != nil {
				return domain.Document{}, domain.ExtractedData{}, gerr
			}
			return cur, extracted, nil
		}
		return domain.Document{}, domain.ExtractedData{}, err
	}
	return finalized, extracted, nil
}

func (s *DocumentService) List(ctx context.Context, ownerID int64, planItemID *int64, page, size int) ([]DocumentView, int, error) {
	if planItemID != nil {
		if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, *planItemID, ownerID); err != nil {
			return nil, 0, err
		}
	}
	offset, limit := offsetLimit(page, size)
	var (
		docs  []domain.Document
		total int
		err   error
	)
	if s.Groups != nil {
		owners, gerr := s.Groups.CoMemberIDs(ctx, ownerID)
		if gerr != nil {
			return nil, 0, gerr
		}
		docs, total, err = s.Docs.ListByOwners(ctx, owners, planItemID, offset, limit)
	} else {
		docs, total, err = s.Docs.ListByOwner(ctx, ownerID, planItemID, offset, limit)
	}
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
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) StorageStatus(ctx context.Context, ownerID, docID int64) (DocumentStorageStatus, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentStorageStatus{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
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
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
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
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
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
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	// Atomic, conditional confirm under SERIALIZABLE: only pending_review may be
	// confirmed. Doing the check inside the transaction (rather than read-then-
	// write here) closes the race where two corporate-account members confirm
	// the same document at once, or a member confirms while the worker is still
	// finalizing it.
	doc, err = s.Docs.UpdateStatusSerializable(ctx, docID, docStatusConfirmed, docStatusPendingReview)
	if err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) Reject(ctx context.Context, ownerID, docID int64) (DocumentView, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentView{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	// Reject any non-confirmed state, atomically under SERIALIZABLE so a
	// concurrent confirm by another member can't be clobbered.
	doc, err = s.Docs.UpdateStatusSerializable(ctx, docID, docStatusRejected,
		docStatusUploaded, docStatusProcessing, docStatusPendingReview, docStatusFailed, docStatusRejected)
	if err != nil {
		return DocumentView{}, err
	}
	return s.viewOf(ctx, doc)
}

func (s *DocumentService) Reanalyze(ctx context.Context, ownerID, docID int64) (DocumentView, error) {
	doc, err := s.Docs.GetByID(ctx, docID)
	if err != nil {
		return DocumentView{}, err
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, doc.PlanItemID, ownerID); err != nil {
		return DocumentView{}, err
	}
	if doc.Status == docStatusConfirmed {
		return DocumentView{}, ErrConflict
	}

	obj, err := s.Store.Open(ctx, doc.FilePath)
	if err != nil {
		return DocumentView{}, err
	}
	defer obj.Body.Close()

	content, err := io.ReadAll(obj.Body)
	if err != nil {
		return DocumentView{}, err
	}
	if doc, err = s.Docs.UpdateStatus(ctx, docID, docStatusProcessing); err != nil {
		return DocumentView{}, err
	}
	doc, extracted, err := s.applyRecognition(ctx, doc, content)
	if err != nil {
		return DocumentView{}, err
	}
	return DocumentView{Doc: doc, Extracted: &extracted}, nil
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
