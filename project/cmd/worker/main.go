package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"diplom.com/m/internal/adapters/nats"
	"diplom.com/m/internal/adapters/pganalysis"
	"diplom.com/m/internal/adapters/pgcore"
	"diplom.com/m/internal/adapters/s3"
	"diplom.com/m/internal/config"
	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/usecase"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	coreStore, err := pgcore.NewStore(ctx, cfg.CoreDBDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer coreStore.Close()

	analysisRepo, err := pganalysis.New(ctx, cfg.AnalysisDBDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer analysisRepo.Close()

	docRepo := pgcore.NewDocumentRepo(coreStore)
	jobRepo := pgcore.NewJobRepo(coreStore)
	broker := nats.NewInMemoryBroker()
	objStore := s3.NewLocalStore(cfg.StorageRootDir, cfg.StorageDownloadRoute)

	processor := &usecase.Processor{
		Docs:        docRepo,
		Jobs:        jobRepo,
		Analysis:    analysisRepo,
		Store:       objStore,
		OCR:         usecase.MockOCR{},
		LLM:         usecase.MockLLM{},
		Broker:      broker,
		MaxAttempts: cfg.WorkerMaxAttempts,
	}

	go func() {
		_ = broker.Subscribe(ctx, "jobs.step.requested", "worker", "", func(evt domain.Event) error {
			return processor.HandleStepRequested(ctx, evt)
		})
	}()
	go func() {
		_ = broker.Subscribe(ctx, "jobs.step.completed", "worker", "", func(evt domain.Event) error {
			return processor.HandleStepCompleted(ctx, evt)
		})
	}()

	ticker := time.NewTicker(cfg.WorkerPollInterval)
	defer ticker.Stop()

	log.Printf("worker started; poll=%s batch=%d", cfg.WorkerPollInterval, cfg.WorkerBatchSize)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := jobRepo.ListCreatedJobs(ctx, cfg.WorkerBatchSize)
			if err != nil {
				log.Printf("list created jobs failed: %v", err)
				continue
			}
			for _, j := range jobs {
				evt := domain.Event{
					EventID:    domain.NewEventID(),
					OccurredAt: time.Now().UTC(),
					Type:       "jobs.created",
					UserID:     j.OwnerID,
					DocumentID: j.DocumentID,
					JobID:      j.ID,
				}
				if err := processor.HandleJobCreated(ctx, evt); err != nil {
					log.Printf("job %s failed start: %v", j.ID, err)
				}
			}
		}
	}
}
