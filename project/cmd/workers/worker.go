package worker

import (
	"context"
	"errors"
	"time"

	"diplom.com/m/internal/ports"

	"diplom.com/m/internal/domain"
)

type Processor struct {
	Docs     ports.DocumentRepo
	Jobs     ports.JobRepo
	Analysis ports.AnalysisRepo
	Store    ports.ObjectStore
	OCR      ports.OCRClient
	LLM      ports.LLMClient
	Broker   ports.Broker

	MaxAttempts int
}

func (p *Processor) HandleJobCreated(ctx context.Context, evt domain.Event) error {
	// set statuses
	_ = p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocProcessing)
	_ = p.Jobs.MarkJobRunning(ctx, evt.JobID)

	// create OCR step and request
	_ = p.Jobs.UpsertStep(ctx, evt.JobID, domain.StepOCR)

	return p.publishStepRequested(ctx, evt, domain.StepOCR)
}

func (p *Processor) HandleStepCompleted(ctx context.Context, evt domain.Event) error {
	if evt.Step == nil {
		return errors.New("missing step")
	}

	switch *evt.Step {
	case domain.StepOCR:
		_ = p.Jobs.UpsertStep(ctx, evt.JobID, domain.StepLLM)
		return p.publishStepRequested(ctx, evt, domain.StepLLM)

	case domain.StepLLM:
		// normalize+save
		fields, _ := evt.Data.(map[string]any) // из LLM mock можно слать map[string]AnalysisField, но через json обычно any
		normalized := map[string]ports.AnalysisField{}
		for k, v := range fields {
			normalized[k] = ports.AnalysisField{Value: toString(v), Confidence: 0.9}
		}
		if err := p.Analysis.SaveExtraction(ctx, evt.UserID, evt.DocumentID, normalized); err != nil {
			_ = p.Jobs.MarkJobFailed(ctx, evt.JobID)
			_ = p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocFailed)
			return err
		}

		_ = p.Jobs.MarkJobCompleted(ctx, evt.JobID)
		_ = p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocReady)

		done := domain.Event{
			EventID:    newUUID(),
			OccurredAt: time.Now().UTC(),
			UserID:     evt.UserID,
			DocumentID: evt.DocumentID,
			JobID:      evt.JobID,
			Type:       "jobs.completed",
		}
		_ = p.Broker.Publish(ctx, "jobs.completed", done)
		_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, done)
		return nil

	default:
		return nil
	}
}

func (p *Processor) HandleStepRequested(ctx context.Context, evt domain.Event) error {
	if evt.Step == nil {
		return errors.New("missing step")
	}
	step := *evt.Step

	// идемпотентность + попытка:
	attempt, err := p.Jobs.MarkStepRunning(ctx, evt.JobID, step)
	if err != nil {
		return err
	}

	// достаём документ из object store
	doc, err := p.Docs.Get(ctx, evt.UserID, evt.DocumentID)
	if err != nil {
		return err
	}
	rc, err := p.Store.Get(ctx, doc.ObjectKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	switch step {
	case domain.StepOCR:
		text, err := p.OCR.ExtractText(ctx, rc)
		if err != nil {
			return p.failStep(ctx, evt, step, attempt, err)
		}
		_ = p.Jobs.MarkStepCompleted(ctx, evt.JobID, step)
		return p.publishStepCompleted(ctx, evt, step, map[string]any{"text": text})

	case domain.StepLLM:
		// OCR text должен прийти как data из предыдущего шага; для MVP можно хранить текст временно в job_steps.meta,
		// но проще: в событии OCRCompleted слать текст, а worker принимать и дальше слать LLMRequested с текстом.
		// Здесь предполагаем, что LLMRequested evt.Data содержит text.
		text := ""
		if m, ok := evt.Data.(map[string]any); ok {
			if v, ok := m["text"].(string); ok {
				text = v
			}
		}
		fields, err := p.LLM.Analyze(ctx, text)
		if err != nil {
			return p.failStep(ctx, evt, step, attempt, err)
		}
		_ = p.Jobs.MarkStepCompleted(ctx, evt.JobID, step)

		anyMap := map[string]any{}
		for k, f := range fields {
			anyMap[k] = f.Value
		}
		return p.publishStepCompleted(ctx, evt, step, anyMap)

	default:
		return nil
	}
}

func (p *Processor) failStep(ctx context.Context, evt domain.Event, step domain.StepName, attempt int, cause error) error {
	attempt, _ = p.Jobs.MarkStepFailed(ctx, evt.JobID, step, cause.Error())
	if p.MaxAttempts > 0 && attempt >= p.MaxAttempts {
		_ = p.Jobs.MarkJobFailed(ctx, evt.JobID)
		_ = p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocFailed)

		failed := domain.Event{EventID: newUUID(), OccurredAt: time.Now().UTC(), UserID: evt.UserID, DocumentID: evt.DocumentID, JobID: evt.JobID, Type: "jobs.failed"}
		_ = p.Broker.Publish(ctx, "jobs.failed", failed)
		_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, failed)
		return cause
	}

	// retry: publish requested again with backoff hint
	return p.publishStepRequested(ctx, evt, step)
}

func (p *Processor) publishStepRequested(ctx context.Context, evt domain.Event, step domain.StepName) error {
	s := step
	attempt := 0
	e := domain.Event{
		EventID:    newUUID(),
		OccurredAt: time.Now().UTC(),
		UserID:     evt.UserID,
		DocumentID: evt.DocumentID,
		JobID:      evt.JobID,
		Type:       "jobs.step.requested",
		Step:       &s,
		Attempt:    &attempt,
		Data:       evt.Data, // например, для LLMRequested прокинем text
	}
	_ = p.Broker.Publish(ctx, "jobs.step.requested", e)
	_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, e)
	return nil
}

func (p *Processor) publishStepCompleted(ctx context.Context, evt domain.Event, step domain.StepName, data any) error {
	s := step
	e := domain.Event{
		EventID:    newUUID(),
		OccurredAt: time.Now().UTC(),
		UserID:     evt.UserID,
		DocumentID: evt.DocumentID,
		JobID:      evt.JobID,
		Type:       "jobs.step.completed",
		Step:       &s,
		Data:       data,
	}
	_ = p.Broker.Publish(ctx, "jobs.step.completed", e)
	_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, e)
	return nil
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func newUUID() string { return time.Now().UTC().Format("20060102150405.000000000") }
