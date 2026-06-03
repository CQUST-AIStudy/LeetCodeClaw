package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"leetcodeclaw/internal/api"
	"leetcodeclaw/internal/config"
	"leetcodeclaw/internal/leetcode"
	"leetcodeclaw/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	httpClient := &http.Client{Timeout: cfg.LeetCode.Timeout}
	client := leetcode.NewClient(httpClient, cfg.LeetCode.Retries)
	problemService := leetcode.NewProblemService(client)

	var store *storage.Store
	if candidate, err := storage.NewMySQLStore(cfg.Database.DSN()); err != nil {
		if cfg.DBRequired {
			log.Fatalf("create mysql store: %v", err)
		}
		log.Printf("mysql store disabled: %v", err)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := candidate.Ping(ctx); err != nil {
			cancel()
			_ = candidate.Close()
			if cfg.DBRequired {
				log.Fatalf("connect mysql: %v", err)
			}
			log.Printf("mysql store disabled: %v", err)
		} else if err := candidate.CheckSchema(ctx); err != nil {
			cancel()
			_ = candidate.Close()
			if cfg.DBRequired {
				log.Fatalf("check mysql schema: %v", err)
			}
			log.Printf("mysql store disabled: %v", err)
		} else {
			cancel()
			store = candidate
			defer store.Close()
		}
	}

	apiServer := api.NewServer(problemService, store)
	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      apiServer.Routes(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		log.Printf("leetcode claw api listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown failed: %v", err)
	}
}
