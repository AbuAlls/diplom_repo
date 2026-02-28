package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	httpapi "diplom.com/m/internal/adapters/httpapi"
	"diplom.com/m/internal/adapters/nats"
	"diplom.com/m/internal/adapters/pganalysis"
	"diplom.com/m/internal/adapters/pgcore"
	"diplom.com/m/internal/adapters/s3"
	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/config"
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
	userRepo := pgcore.NewUserRepo(coreStore)
	sessionRepo := pgcore.NewSessionRepo(coreStore)
	broker := nats.NewInMemoryBroker()
	objStore := s3.NewLocalStore(cfg.StorageRootDir, cfg.StorageDownloadRoute)

	docsSvc := &usecase.DocumentService{Docs: docRepo, Jobs: jobRepo, Store: objStore, Broker: broker}
	authSvc := &usecase.AuthService{
		Users:      userRepo,
		Sessions:   sessionRepo,
		Tokens:     auth.TokenManager{Secret: []byte(cfg.JWTSecret), Issuer: cfg.JWTIssuer},
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
	}

	api := &httpapi.API{
		Auth:     authSvc,
		Docs:     docsSvc,
		DocRepo:  docRepo,
		JobRepo:  jobRepo,
		Analysis: analysisRepo,
		Store:    objStore,
		SSE:      &httpapi.SSEHandler{Broker: broker},
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
