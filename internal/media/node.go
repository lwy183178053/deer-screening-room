package media

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type NodeConfig struct {
	Name, PublicURL, GatewayURL, NodeAPIToken, RelayToken string
	MediaRoot, PosterRoot                                 string
	HTTPClient                                            *http.Client
	MaxStreams                                            int
}

type Node struct {
	config             NodeConfig
	scanner            *Scanner
	streamSlots        chan struct{}
	stateMu            sync.RWMutex
	scanning           bool
	lastScanAt         time.Time
	lastScanError      string
	lastSyncedRevision string
}

func NewNode(config NodeConfig) (*Node, error) {
	if config.Name == "" || config.PublicURL == "" || config.GatewayURL == "" || config.NodeAPIToken == "" || config.RelayToken == "" {
		return nil, errors.New("node configuration is incomplete")
	}
	if config.HTTPClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
		transport.MaxIdleConns = 16
		transport.MaxIdleConnsPerHost = 8
		transport.IdleConnTimeout = 90 * time.Second
		transport.TLSHandshakeTimeout = 10 * time.Second
		transport.ResponseHeaderTimeout = 15 * time.Second
		config.HTTPClient = &http.Client{Transport: transport, Timeout: 5 * time.Minute}
	}
	if config.MaxStreams < 1 {
		config.MaxStreams = 64
	}
	scanner, err := NewScanner(config.MediaRoot, config.PosterRoot)
	if err != nil {
		return nil, err
	}
	return &Node{config: config, scanner: scanner, streamSlots: make(chan struct{}, config.MaxStreams)}, nil
}

func (n *Node) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "service": "media-node"})
	})
	mux.HandleFunc("GET /internal/media/{key}", n.serveMedia)
	mux.HandleFunc("HEAD /internal/media/{key}", n.serveMedia)
	mux.HandleFunc("GET /internal/posters/{key}", n.servePoster)
	mux.HandleFunc("HEAD /internal/posters/{key}", n.servePoster)
	mux.HandleFunc("POST /internal/rescan", n.rescan)
	return mux
}

func (n *Node) Start(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go n.heartbeatLoop(ctx)
	if err := n.rescanAndSync(ctx); err != nil {
		fmt.Printf("initial media scan: %v\n", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := n.rescanAndSync(ctx); err != nil {
				fmt.Printf("media scan: %v\n", err)
			}
		}
	}
}

func (n *Node) heartbeatLoop(ctx context.Context) {
	_ = n.heartbeat(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := n.heartbeat(ctx); err != nil {
				fmt.Printf("media heartbeat: %v\n", err)
			}
		}
	}
}

func (n *Node) rescan(w http.ResponseWriter, r *http.Request) {
	if !n.authorized(r) {
		writeError(w, 401, "relay authentication required")
		return
	}
	if !n.beginScan() {
		writeError(w, http.StatusConflict, "scan already in progress")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		_ = n.runScan(ctx)
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scanning"})
}

func (n *Node) rescanAndSync(ctx context.Context) error {
	if !n.beginScan() {
		return errors.New("scan already in progress")
	}
	return n.runScan(ctx)
}

func (n *Node) beginScan() bool {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	if n.scanning {
		return false
	}
	n.scanning = true
	return true
}

func (n *Node) runScan(ctx context.Context) (scanErr error) {
	defer func() {
		n.stateMu.Lock()
		n.scanning = false
		n.lastScanAt = time.Now()
		if scanErr == nil {
			n.lastScanError = ""
		} else {
			n.lastScanError = scanErr.Error()
			if len(n.lastScanError) > 500 {
				n.lastScanError = n.lastScanError[:500]
			}
		}
		n.stateMu.Unlock()
	}()
	items, revision, err := n.scanner.ScanWithRevision()
	if err != nil {
		return err
	}
	n.stateMu.RLock()
	unchanged := revision == n.lastSyncedRevision
	n.stateMu.RUnlock()
	if unchanged {
		return nil
	}
	total, available, err := Capacity(n.config.MediaRoot)
	if err != nil {
		total, available = 0, 0
	}
	scanAt := time.Now()
	payload := map[string]any{"node_name": n.config.Name, "base_url": n.config.PublicURL, "total_bytes": total, "available_bytes": available, "items": items, "inventory_revision": revision, "scan_status": "ok", "last_scan_at": scanAt}
	if err := n.postGateway(ctx, "/api/v1/internal/media/sync", payload); err != nil {
		return err
	}
	n.stateMu.Lock()
	n.lastSyncedRevision = revision
	n.lastScanAt = scanAt
	n.stateMu.Unlock()
	return nil
}

func (n *Node) heartbeat(ctx context.Context) error {
	total, available, err := Capacity(n.config.MediaRoot)
	if err != nil {
		total, available = 0, 0
	}
	status, lastScanAt, scanError, revision := n.scanState()
	return n.postGateway(ctx, "/api/v1/internal/media/heartbeat", map[string]any{"node_name": n.config.Name, "base_url": n.config.PublicURL, "total_bytes": total, "available_bytes": available, "scan_status": status, "last_scan_at": lastScanAt, "scan_error": scanError, "inventory_revision": revision})
}

func (n *Node) scanState() (string, time.Time, string, string) {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	status := "pending"
	if n.scanning {
		status = "scanning"
	} else if n.lastScanError != "" {
		status = "error"
	} else if !n.lastScanAt.IsZero() {
		status = "ok"
	}
	return status, n.lastScanAt, n.lastScanError, n.lastSyncedRevision
}

func (n *Node) postGateway(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(n.config.GatewayURL, "/")+path, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+n.config.NodeAPIToken)
	response, err := n.config.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("gateway status %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func (n *Node) serveMedia(w http.ResponseWriter, r *http.Request) {
	if !n.authorized(r) {
		writeError(w, 401, "relay authentication required")
		return
	}
	path, ok := n.scanner.ResolveMedia(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		select {
		case n.streamSlots <- struct{}{}:
			defer func() { <-n.streamSlots }()
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, "media node is busy")
			return
		}
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "media unavailable", 500)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "media unavailable", 500)
		return
	}
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Content-Type", mediaType(filepath.Ext(path)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func (n *Node) servePoster(w http.ResponseWriter, r *http.Request) {
	if !n.authorized(r) {
		writeError(w, 401, "relay authentication required")
		return
	}
	path, ok := n.scanner.ResolvePoster(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}
func (n *Node) authorized(r *http.Request) bool {
	received := r.Header.Get("X-Relay-Token")
	return len(received) == len(n.config.RelayToken) && received != "" && subtle.ConstantTimeCompare([]byte(received), []byte(n.config.RelayToken)) == 1
}
func mediaType(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4", ".m4v", ".mov":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
