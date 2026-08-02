package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type NodeCredential struct {
	APIToken string `json:"api_token"`
	BaseURL  string `json:"base_url"`
}

type Config struct {
	Role              string
	HTTPAddr          string
	DatabaseURL       string
	CookieSecure      bool
	SessionTTL        time.Duration
	BootstrapAdmin    string
	BootstrapPassword string
	NodeAPIToken      string
	MediaRoot         string
	PosterRoot        string
	NodeName          string
	NodePublicURL     string
	GatewayURL        string
	ScanInterval      time.Duration
	PasswordHashJobs  int
	UserStreamRPM     int
	TurnURLs          []string
	StunURLs          []string
	TurnSecret        string
	TurnTTL           time.Duration
	NodeCredentials   map[string]NodeCredential
}

func Load() (Config, error) {
	nodeCredentials, err := parseNodeCredentials(os.Getenv("DEER_NODE_CREDENTIALS"))
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Role:              value("DEER_ROLE", "gateway"),
		HTTPAddr:          value("DEER_HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		CookieSecure:      boolean("DEER_COOKIE_SECURE", false),
		SessionTTL:        duration("DEER_SESSION_TTL", 7*24*time.Hour),
		BootstrapAdmin:    strings.ToLower(strings.TrimSpace(value("DEER_BOOTSTRAP_ADMIN", "admin@example.com"))),
		BootstrapPassword: os.Getenv("DEER_BOOTSTRAP_PASSWORD"),
		NodeAPIToken:      os.Getenv("DEER_NODE_API_TOKEN"),
		MediaRoot:         value("DEER_MEDIA_ROOT", "/media"),
		PosterRoot:        value("DEER_POSTER_ROOT", "/app/posters"),
		NodeName:          value("DEER_NODE_NAME", "fnos-media"),
		NodePublicURL:     strings.TrimRight(value("DEER_NODE_PUBLIC_URL", "http://media-node:8081"), "/"),
		GatewayURL:        strings.TrimRight(value("DEER_GATEWAY_URL", "http://gateway:8080"), "/"),
		ScanInterval:      duration("DEER_SCAN_INTERVAL", 10*time.Minute),
		PasswordHashJobs:  int(integer("DEER_PASSWORD_HASH_CONCURRENCY", 4)),
		UserStreamRPM:     int(integer("DEER_USER_STREAM_REQUESTS_PER_MINUTE", 120)),
		TurnURLs:          splitCSV(os.Getenv("DEER_TURN_URLS")),
		StunURLs:          splitCSV(os.Getenv("DEER_STUN_URLS")),
		TurnSecret:        os.Getenv("DEER_TURN_SECRET"),
		TurnTTL:           duration("DEER_TURN_TTL", 2*time.Hour),
		NodeCredentials:   nodeCredentials,
	}
	if cfg.Role == "gateway" && cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required for gateway")
	}
	if cfg.Role == "gateway" && len(cfg.NodeCredentials) == 0 && cfg.NodeAPIToken == "" {
		return Config{}, errors.New("DEER_NODE_CREDENTIALS or legacy node tokens are required for gateway")
	}
	if cfg.Role == "gateway" && (len(cfg.TurnURLs) == 0 || cfg.TurnSecret == "") {
		return Config{}, errors.New("DEER_TURN_URLS and DEER_TURN_SECRET are required for gateway")
	}
	for _, turnURL := range cfg.TurnURLs {
		lower := strings.ToLower(turnURL)
		if !strings.HasPrefix(lower, "turn:") && !strings.HasPrefix(lower, "turns:") {
			return Config{}, errors.New("DEER_TURN_URLS must contain only TURN relay URLs")
		}
	}
	for _, stunURL := range cfg.StunURLs {
		lower := strings.ToLower(stunURL)
		if !strings.HasPrefix(lower, "stun:") {
			return Config{}, errors.New("DEER_STUN_URLS must contain only STUN URLs")
		}
	}
	if cfg.Role == "media-node" && cfg.NodeAPIToken == "" {
		return Config{}, errors.New("DEER_NODE_API_TOKEN is required")
	}
	return cfg, nil
}

func splitCSV(raw string) []string {
	var result []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseNodeCredentials(raw string) (map[string]NodeCredential, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	result := make(map[string]NodeCredential)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, errors.New("DEER_NODE_CREDENTIALS must be valid JSON")
	}
	for name, credential := range result {
		if name != strings.TrimSpace(name) {
			return nil, errors.New("DEER_NODE_CREDENTIALS node names cannot contain surrounding whitespace")
		}
		credential.BaseURL = strings.TrimRight(strings.TrimSpace(credential.BaseURL), "/")
		if strings.TrimSpace(name) == "" || credential.APIToken == "" || credential.BaseURL == "" {
			return nil, errors.New("every DEER_NODE_CREDENTIALS entry requires name, api_token and base_url")
		}
		result[name] = credential
	}
	return result, nil
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
