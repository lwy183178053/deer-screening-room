package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Role                    string
	HTTPAddr                string
	DatabaseURL             string
	CookieSecure            bool
	SessionTTL              time.Duration
	BootstrapAdmin          string
	BootstrapPassword       string
	NodeAPIToken            string
	RelayToken              string
	MediaRoot               string
	PosterRoot              string
	NodeName                string
	NodePublicURL           string
	GatewayURL              string
	ScanInterval            time.Duration
	PasswordHashJobs        int
	UserStreamRPM           int
	NodeSecretsKey          []byte
	WGProvisionerURL        string
	WGProvisionerToken      string
	CloudWireGuardPublicKey string
	CloudEndpoint           string
	NodeImage               string
	NodeVersion             string
}

func Load() (Config, error) {
	nodeSecretsKey, err := parseNodeSecretsKey(os.Getenv("DEER_NODE_SECRETS_KEY"))
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Role:                    value("DEER_ROLE", "gateway"),
		HTTPAddr:                value("DEER_HTTP_ADDR", ":8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		CookieSecure:            boolean("DEER_COOKIE_SECURE", false),
		SessionTTL:              duration("DEER_SESSION_TTL", 7*24*time.Hour),
		BootstrapAdmin:          strings.ToLower(strings.TrimSpace(value("DEER_BOOTSTRAP_ADMIN", "admin@example.com"))),
		BootstrapPassword:       os.Getenv("DEER_BOOTSTRAP_PASSWORD"),
		NodeAPIToken:            os.Getenv("DEER_NODE_API_TOKEN"),
		RelayToken:              os.Getenv("DEER_RELAY_TOKEN"),
		MediaRoot:               value("DEER_MEDIA_ROOT", "/media"),
		PosterRoot:              value("DEER_POSTER_ROOT", "/app/posters"),
		NodeName:                value("DEER_NODE_NAME", "fnos-media"),
		NodePublicURL:           strings.TrimRight(value("DEER_NODE_PUBLIC_URL", "http://media-node:8081"), "/"),
		GatewayURL:              strings.TrimRight(value("DEER_GATEWAY_URL", "http://gateway:8080"), "/"),
		ScanInterval:            duration("DEER_SCAN_INTERVAL", 10*time.Minute),
		PasswordHashJobs:        int(integer("DEER_PASSWORD_HASH_CONCURRENCY", 4)),
		UserStreamRPM:           int(integer("DEER_USER_STREAM_REQUESTS_PER_MINUTE", 120)),
		NodeSecretsKey:          nodeSecretsKey,
		WGProvisionerURL:        strings.TrimRight(value("DEER_WG_PROVISIONER_URL", "http://127.0.0.1:9191"), "/"),
		WGProvisionerToken:      os.Getenv("DEER_WG_PROVISIONER_TOKEN"),
		CloudWireGuardPublicKey: strings.TrimSpace(os.Getenv("DEER_CLOUD_WIREGUARD_PUBLIC_KEY")),
		CloudEndpoint:           strings.TrimSpace(os.Getenv("DEER_CLOUD_ENDPOINT")),
		NodeImage:               value("DEER_NODE_IMAGE", "ghcr.io/lwy183178053/deer-screening-room"),
		NodeVersion:             value("DEER_NODE_VERSION", "v0.1.4"),
	}
	if cfg.Role == "gateway" && cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required for gateway")
	}
	if cfg.Role == "gateway" && len(cfg.NodeSecretsKey) != 32 {
		return Config{}, errors.New("DEER_NODE_SECRETS_KEY is required for gateway")
	}
	if cfg.Role == "media-node" && (cfg.NodeAPIToken == "" || cfg.RelayToken == "") {
		return Config{}, errors.New("DEER_NODE_API_TOKEN and DEER_RELAY_TOKEN are required")
	}
	return cfg, nil
}

func parseNodeSecretsKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, decode := range []func(string) ([]byte, error){base64.RawStdEncoding.DecodeString, base64.StdEncoding.DecodeString, hex.DecodeString} {
		key, err := decode(raw)
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}
	return nil, errors.New("DEER_NODE_SECRETS_KEY must encode exactly 32 bytes")
}

func value(name, fallback string) string {
	if result := strings.TrimSpace(os.Getenv(name)); result != "" {
		return result
	}
	return fallback
}

func boolean(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func duration(name string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv(name)))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func integer(name string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(name)), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
