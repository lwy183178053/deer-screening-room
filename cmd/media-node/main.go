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
	"deerroom/internal/media"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	node, err := media.NewNode(media.NodeConfig{Name: cfg.NodeName, PublicURL: cfg.NodePublicURL, GatewayURL: cfg.GatewayURL, NodeAPIToken: cfg.NodeAPIToken, RelayToken: cfg.RelayToken, MediaSourceRoot: cfg.MediaSourceRoot, MediaRoot: cfg.MediaRoot, PosterRoot: cfg.PosterRoot})
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: node.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 1 << 20}
	go func() {
		if err := node.Start(ctx, cfg.ScanInterval); err != nil {
			log.Printf("node loop: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("media node listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
