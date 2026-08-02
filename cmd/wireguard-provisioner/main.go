package main

import (
	"log"
	"net/http"
	"os"

	"deerroom/internal/wireguard"
)

func main() {
	configPath := value("DEER_WG_CONFIG", "/config/wg_confs/wg0.conf")
	server := &wireguard.Server{
		ConfigPath: configPath,
		Interface:  value("DEER_WG_INTERFACE", "wg0"),
		Token:      os.Getenv("DEER_WG_PROVISIONER_TOKEN"),
	}
	if server.Token == "" {
		log.Fatal("DEER_WG_PROVISIONER_TOKEN is required")
	}
	log.Printf("wireguard provisioner listening on 127.0.0.1:9191")
	if err := http.ListenAndServe("127.0.0.1:9191", server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func value(name, fallback string) string {
	if result := os.Getenv(name); result != "" {
		return result
	}
	return fallback
}
