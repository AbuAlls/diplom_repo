package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type Processor struct {
	Docs        ports.DocumentRepo
	Jobs        ports.JobRepo
	Analysis    ports.AnalysisRepo
	Store       ports.ObjectStore
	OCR         ports.OCRClient
	LLM         ports.LLMClient
	Broker      ports.Broker
	MaxAttempts int
}

func (p *Processor) HandleJobCreated(ctx context.Context, evt domain.Event) error {
	if err := p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocProcessing); err != nil {
		return err
	}
	if err := p.Jobs.MarkJobRunning(ctx, evt.JobID); err != nil {
		return err
	}
	if err := p.Jobs.UpsertStep(ctx, evt.JobID, domain.StepOCR); err != nil {
		return err
	}
	return p.publishStepRequested(ctx, evt, domain.StepOCR, 1, nil)
}

func (p *Processor) HandleStepCompleted(ctx context.Context, evt domain.Event) error {
	if evt.Step == nil {
		return errors.New("missing step")
	}
	switch *evt.Step {
	case domain.StepOCR:
		text := ""
		if m, ok := evt.Data.(map[string]any); ok {
			if v, ok := m["text"].(string); ok {
				text = v
			}
		}
		if err := p.Jobs.UpsertStep(ctx, evt.JobID, domain.StepLLM); err != nil {
			return err
		}
		return p.publishStepRequested(ctx, evt, domain.StepLLM, 1, map[string]any{"text": text})
	case domain.StepLLM:
		fields, _ := evt.Data.(map[string]any)
		normalized := map[string]ports.AnalysisField{}
		for k, v := range fields {
			normalized[k] = ports.AnalysisField{Value: toString(v), Confidence: 0.9}
		}
		if err := p.Analysis.SaveExtraction(ctx, evt.UserID, evt.DocumentID, normalized); err != nil {
			_ = p.Jobs.MarkJobFailed(ctx, evt.JobID)
			_ = p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocFailed)
			return err
		}
		if err := p.Jobs.MarkJobCompleted(ctx, evt.JobID); err != nil {
			return err
		}
		if err := p.Docs.UpdateStatus(ctx, evt.DocumentID, domain.DocReady); err != nil {
			return err
		}
		done := domain.Event{
			EventID:    domain.NewEventID(),
			OccurredAt: time.Now().UTC(),
			Type:       "jobs.completed",
			UserID:     evt.UserID,
			DocumentID: evt.DocumentID,
			JobID:      evt.JobID,
		}
		if err := p.Broker.Publish(ctx, "jobs.completed", done); err != nil {
			return err
		}
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
	attempt, err := p.Jobs.MarkStepRunning(ctx, evt.JobID, step)
	if err != nil {
		return err
	}
	job, err := p.Jobs.GetByID(ctx, evt.JobID)
	if err != nil {
		return err
	}
	doc, err := p.Docs.Get(ctx, job.OwnerID, evt.DocumentID)
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
		if err := p.Jobs.MarkStepCompleted(ctx, evt.JobID, step); err != nil {
			return err
		}
		return p.publishStepCompleted(ctx, evt, step, map[string]any{"text": text})
	case domain.StepLLM:
		text := ""
		if m, ok := evt.Data.(map[string]any); ok {
			if v, ok := m["text"].(string); ok {
				text = v
			}
		}
		if text == "" {
			b, _ := io.ReadAll(io.LimitReader(rc, 64*1024))
			text = string(b)
		}
		fields, err := p.LLM.Analyze(ctx, text)
		if err != nil {
			return p.failStep(ctx, evt, step, attempt, err)
		}
		if err := p.Jobs.MarkStepCompleted(ctx, evt.JobID, step); err != nil {
			return err
		}
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
		failed := domain.Event{
			EventID:    domain.NewEventID(),
			OccurredAt: time.Now().UTC(),
			Type:       "jobs.failed",
			UserID:     evt.UserID,
			DocumentID: evt.DocumentID,
			JobID:      evt.JobID,
		}
		_ = p.Broker.Publish(ctx, "jobs.failed", failed)
		_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, failed)
		return cause
	}
	return p.publishStepRequested(ctx, evt, step, attempt+1, evt.Data)
}

func (p *Processor) publishStepRequested(ctx context.Context, evt domain.Event, step domain.StepName, attempt int, data any) error {
	s := step
	e := domain.Event{
		EventID:    domain.NewEventID(),
		OccurredAt: time.Now().UTC(),
		Type:       "jobs.step.requested",
		UserID:     evt.UserID,
		DocumentID: evt.DocumentID,
		JobID:      evt.JobID,
		Step:       &s,
		Attempt:    &attempt,
		Data:       data,
	}
	if err := p.Broker.Publish(ctx, "jobs.step.requested", e); err != nil {
		return err
	}
	_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, e)
	return nil
}

func (p *Processor) publishStepCompleted(ctx context.Context, evt domain.Event, step domain.StepName, data any) error {
	s := step
	e := domain.Event{
		EventID:    domain.NewEventID(),
		OccurredAt: time.Now().UTC(),
		Type:       "jobs.step.completed",
		UserID:     evt.UserID,
		DocumentID: evt.DocumentID,
		JobID:      evt.JobID,
		Step:       &s,
		Data:       data,
	}
	if err := p.Broker.Publish(ctx, "jobs.step.completed", e); err != nil {
		return err
	}
	_ = p.Broker.PublishUI(ctx, "ui.jobs."+evt.JobID, e)
	return nil
}

type MockOCR struct{}

func (MockOCR) ExtractText(ctx context.Context, doc io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(doc, 1<<20))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

type MockLLM struct{}

func (MockLLM) Analyze(ctx context.Context, text string) (map[string]ports.AnalysisField, error) {
	words := len(strings.Fields(text))
	return map[string]ports.AnalysisField{
		"summary": {
			Value:      fmt.Sprintf("Document contains %d words", words),
			Confidence: 0.8,
		},
		"preview": {
			Value:      truncate(text, 120),
			Confidence: 0.7,
		},
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
