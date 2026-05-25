package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cs2demo/platform/internal/analyzer"
	"github.com/cs2demo/platform/internal/api"
	"github.com/cs2demo/platform/internal/config"
	"github.com/cs2demo/platform/internal/orchestrator"
	"github.com/cs2demo/platform/internal/parser"
	"github.com/cs2demo/platform/internal/prokb"
	"github.com/cs2demo/platform/internal/storage"
)

func main() {
	cfg := config.Load()
	log.Printf("config: addr=%s data=%s llm_provider=%s llm_model=%s llm_configured=%v",
		cfg.HTTPAddr, cfg.DataDir, cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey != "")

	store, err := storage.Open(cfg.SQLitePath, cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	kb := prokb.New()
	parsr := parser.New()
	anlz := analyzer.New(cfg.LLMProvider, cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel, kb)
	log.Printf("analyzer provider: %s", anlz.ProviderName())
	orch := orchestrator.New(store, parsr, anlz, cfg.WorkerCount)

	srv := &api.Server{
		Store:          store,
		Orch:           orch,
		MaxUploadBytes: int64(cfg.MaxUploadMB) << 20,
		WebDir:         "./web",
	}

	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv.Router(),
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	orch.Shutdown()
	log.Printf("bye")
}
