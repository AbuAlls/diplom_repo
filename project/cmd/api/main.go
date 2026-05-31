package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"diplom.com/m/internal/adapters/auditai"
	httpapi "diplom.com/m/internal/adapters/httpapi"
	"diplom.com/m/internal/adapters/pganalysis"
	"diplom.com/m/internal/adapters/pgcore"
	"diplom.com/m/internal/adapters/recognition"
	"diplom.com/m/internal/adapters/storage"
	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/config"
	"diplom.com/m/internal/ports"
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

	analysisStore, err := pganalysis.NewStore(ctx, cfg.AnalysisDBDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer analysisStore.Close()

	userRepo := pgcore.NewUserRepo(coreStore)
	planRepo := pgcore.NewPlanRepo(coreStore)
	goalRepo := pgcore.NewGoalRepo(coreStore)
	itemRepo := pgcore.NewPlanItemRepo(coreStore)
	folderRepo := pgcore.NewFolderRepo(coreStore)
	docRepo := pgcore.NewDocumentRepo(coreStore)
	extractedRepo := pganalysis.NewExtractedDataRepo(analysisStore)
	queryRepo := pgcore.NewAnalyticsQueryRepo(coreStore)
	fileStore, err := storage.NewS3Store(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3Region, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		log.Fatal(err)
	}

	// The AI client serves both the analytics agent (Analyzer) and, when
	// RECOGNIZER=audit, document recognition; otherwise the deterministic mock.
	aiClient := auditai.NewClient(cfg.AIServiceURL, cfg.AIModel, cfg.AIRequestTimeout)
	var recognizer ports.Recognizer = recognition.New()
	if cfg.RecognizerKind == "audit" {
		recognizer = &auditai.Recognizer{Client: aiClient}
		log.Printf("recognizer: audit AI service at %s (model %s)", cfg.AIServiceURL, cfg.AIModel)
	} else {
		log.Printf("recognizer: mock")
	}

	authSvc := &usecase.AuthService{
		Users:      userRepo,
		Tokens:     auth.TokenManager{Secret: []byte(cfg.JWTSecret), Issuer: cfg.JWTIssuer},
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
	}
	planSvc := &usecase.PlanService{Plans: planRepo}
	goalSvc := &usecase.GoalService{Goals: goalRepo, Plans: planRepo}
	itemSvc := &usecase.PlanItemService{Items: itemRepo, Goals: goalRepo, Plans: planRepo}
	docSvc := &usecase.DocumentService{
		Docs:       docRepo,
		Folders:    folderRepo,
		Items:      itemRepo,
		Goals:      goalRepo,
		Plans:      planRepo,
		Store:      fileStore,
		Extracted:  extractedRepo,
		Recognizer: recognizer,
	}
	analyticsSvc := &usecase.AnalyticsService{Items: itemRepo, Goals: goalRepo, Plans: planRepo, Docs: docRepo, Analyzer: aiClient}
	internalAnalyticsSvc := &usecase.InternalAnalyticsService{Query: queryRepo}

	api := &httpapi.API{
		Auth:              authSvc,
		Plans:             planSvc,
		Goals:             goalSvc,
		Items:             itemSvc,
		Docs:              docSvc,
		Analytics:         analyticsSvc,
		InternalAnalytics: internalAnalyticsSvc,
		InternalToken:     cfg.InternalAPIToken,
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("api listening on %s", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
