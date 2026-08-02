package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deerroom/internal/config"
	"deerroom/internal/httpapi"
	"deerroom/internal/password"
	"deerroom/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	if cfg.BootstrapPassword != "" {
		hash, err := password.Hash(cfg.BootstrapPassword)
		if err != nil {
			log.Fatal(err)
		}
		if err := database.EnsureAdmin(ctx, cfg.BootstrapAdmin, hash); err != nil {
			log.Fatal(err)
		}
	}
	nodeCredentials := make(map[string]httpapi.NodeCredential, len(cfg.NodeCredentials))
	for name, credential := range cfg.NodeCredentials {
		nodeCredentials[name] = httpapi.NodeCredential{APIToken: credential.APIToken, RelayToken: credential.RelayToken, BaseURL: credential.BaseURL}
	}
	handler := httpapi.New(httpapi.Options{
		Store: database, CookieSecure: cfg.CookieSecure, SessionTTL: cfg.SessionTTL,
		NodeAPIToken: cfg.NodeAPIToken, RelayToken: cfg.RelayToken,
		PasswordHashConcurrency: cfg.PasswordHashJobs, UserStreamRPM: cfg.UserStreamRPM,
		NodeCredentials: nodeCredentials,
	})
	if err := database.CleanupExpired(ctx, time.Now()); err != nil {
		log.Printf("initial maintenance: %v", err)
	}
	go runMaintenance(ctx, database)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("小鹿放映室 gateway listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runMaintenance(ctx context.Context, database *store.Postgres) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := database.CleanupExpired(ctx, now); err != nil {
				log.Printf("maintenance: %v", err)
			}
		}
	}
}
