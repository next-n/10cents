package main

import (
	"context"
	"gateway/internal/config"
	httpx "gateway/internal/http"
	"gateway/internal/outbox"
	"gateway/internal/repo"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	dbPool, err := repo.NewPool(ctx, cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer dbPool.Close()

	// Start outbox worker (webhook sender)
	worker := outbox.NewWorker(dbPool, cfg.WebhookSecretValue())

	worker.PollInterval = 500 * time.Millisecond
	worker.BatchSize = 20
	go worker.Run(ctx)

	router := httpx.NewRouter(dbPool)

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("gateway starting on", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("gateway stopped")
}
