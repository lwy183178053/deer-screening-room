package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestScannerCatalogAndCache(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "工作室甲"), 0o755); err != nil {
		t.Fatal(err)
	}
	videoPath := filepath.Join(root, "工作室甲", "作品一.mp4")
	if err := os.WriteFile(videoPath, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "忽略.zip"), []byte("zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScanner(root, posters)
	if err != nil {
		t.Fatal(err)
	}
	probes := 0
	scanner.probe = func(string) (probeResult, error) {
		probes++
		return probeResult{DurationMS: 123000, BitRate: 2500000, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac"}, nil
	}
	scanner.poster = func(_, destination string) error { return os.WriteFile(destination, []byte("jpeg"), 0o600) }
	items, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	item := items[0]
	if item.Studio != "工作室甲" || item.Title != "作品一" || item.Compatibility != "ready" || item.PosterKey == "" {
		t.Fatalf("item=%+v", item)
	}
	expectedPath := filepath.Join(root, "工作室甲", hashedMediaFilename("工作室甲/作品一.mp4"))
	if resolved, ok := scanner.ResolveMedia(item.MediaKey); !ok || resolved != expectedPath {
		t.Fatalf("resolved=%q ok=%v expected=%q", resolved, ok, expectedPath)
	}
	if _, ok := scanner.ResolvePoster("../../etc/passwd"); ok {
		t.Fatal("accepted traversal poster key")
	}
	if _, err := scanner.Scan(); err != nil {
		t.Fatal(err)
	}
	if probes != 1 {
		t.Fatalf("unchanged file probed %d times", probes)
	}
	_, firstRevision, err := scanner.ScanWithRevision()
	if err != nil || firstRevision == "" {
		t.Fatalf("first revision=%q err=%v", firstRevision, err)
	}
	_, secondRevision, err := scanner.ScanWithRevision()
	if err != nil || secondRevision != firstRevision {
		t.Fatalf("stable revision=%q/%q err=%v", firstRevision, secondRevision, err)
	}
}

func TestScannerAcceptsAV1MP4(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	videoPath := filepath.Join(root, "av1.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScanner(root, posters)
	if err != nil {
		t.Fatal(err)
	}
	scanner.probe = func(string) (probeResult, error) {
		return probeResult{DurationMS: 1000, Width: 1280, Height: 720, VideoCodec: "av1", AudioCodec: "aac"}, nil
	}
	scanner.poster = func(string, string) error { return nil }
	items, err := scanner.Scan()
	if err != nil || len(items) != 1 || items[0].Compatibility != "ready" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestScannerRefreshesCachedCompatibility(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	videoPath := filepath.Join(root, hashedMediaFilename("cached.mp4"))
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := []cachedItem{{Item: Item{MediaKey: mediaKey(filepath.Base(videoPath)), VideoCodec: "av1", AudioCodec: "aac", Compatibility: "unsupported", SizeBytes: info.Size()}, RelativePath: filepath.Base(videoPath), ModifiedUnix: info.ModTime().Unix()}}
	body, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(posters, "catalog-cache.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScanner(root, posters)
	if err != nil {
		t.Fatal(err)
	}
	scanner.probe = func(string) (probeResult, error) {
		t.Fatal("cached media should not be probed")
		return probeResult{}, nil
	}
	items, err := scanner.Scan()
	if err != nil || len(items) != 1 || items[0].Compatibility != "ready" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestScannerUsesMappedTitleForHashedFile(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	studio := filepath.Join(root, "悠米")
	if err := os.Mkdir(studio, 0o755); err != nil {
		t.Fatal(err)
	}
	name := hashedMediaFilename("悠米/作品一.mp4")
	if err := os.WriteFile(filepath.Join(studio, name), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveMediaNameMap(root, mediaNameMap{Version: 1, Files: map[string]mediaNameEntry{
		name: {Studio: "悠米", OriginalName: "作品一.mp4", Title: "作品一"},
	}}); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScanner(root, posters)
	if err != nil {
		t.Fatal(err)
	}
	scanner.probe = func(string) (probeResult, error) {
		return probeResult{DurationMS: 1000, Width: 1280, Height: 720, VideoCodec: "av1", AudioCodec: "aac"}, nil
	}
	scanner.poster = func(string, string) error { return nil }
	items, err := scanner.Scan()
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].Title != "作品一" || items[0].Studio != "悠米" {
		t.Fatalf("item=%+v", items[0])
	}
}

func TestScannerNormalizesNewMedia(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	studio := filepath.Join(root, "工作室甲")
	if err := os.Mkdir(studio, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(studio, "作品一.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScanner(root, posters)
	if err != nil {
		t.Fatal(err)
	}
	scanner.probe = func(string) (probeResult, error) {
		return probeResult{DurationMS: 1000, Width: 1280, Height: 720, VideoCodec: "av1", AudioCodec: "aac"}, nil
	}
	scanner.poster = func(string, string) error { return nil }
	items, err := scanner.Scan()
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	expected := filepath.Join(studio, hashedMediaFilename("工作室甲/作品一.mp4"))
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source was not renamed: %v", err)
	}
	if resolved, ok := scanner.ResolveMedia(items[0].MediaKey); !ok || resolved != expected {
		t.Fatalf("resolved=%q ok=%v expected=%q", resolved, ok, expected)
	}
	if items[0].Title != "作品一" || items[0].Studio != "工作室甲" {
		t.Fatalf("item=%+v", items[0])
	}
}

func TestNodeSkipsUnchangedInventorySync(t *testing.T) {
	var syncRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/internal/media/sync" {
			syncRequests.Add(1)
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["inventory_revision"] == "" {
				http.Error(w, "invalid inventory", http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.mp4"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	node, err := NewNode(NodeConfig{Name: "node", PublicURL: "http://node", GatewayURL: server.URL, NodeAPIToken: "node-token", RelayToken: "relay-token", MediaRoot: root, PosterRoot: t.TempDir(), HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	node.scanner.probe = func(string) (probeResult, error) {
		return probeResult{DurationMS: 1000, VideoCodec: "h264", AudioCodec: "aac"}, nil
	}
	node.scanner.poster = func(string, string) error { return nil }
	if err := node.rescanAndSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := node.rescanAndSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if syncRequests.Load() != 1 {
		t.Fatalf("sync requests=%d", syncRequests.Load())
	}
	status, lastScanAt, scanError, revision := node.scanState()
	if status != "ok" || lastScanAt.IsZero() || scanError != "" || revision == "" {
		t.Fatalf("state=%s/%s/%q/%q", status, lastScanAt, scanError, revision)
	}
}

func TestNodeRangeAndRelayAuthentication(t *testing.T) {
	root := t.TempDir()
	posters := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.mp4"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	node, err := NewNode(NodeConfig{Name: "node", PublicURL: "http://node", GatewayURL: "http://gateway", NodeAPIToken: "node-token", RelayToken: "relay-token", MediaRoot: root, PosterRoot: posters, HTTPClient: &http.Client{}})
	if err != nil {
		t.Fatal(err)
	}
	node.scanner.probe = func(string) (probeResult, error) {
		return probeResult{DurationMS: 1000, VideoCodec: "h264", AudioCodec: "aac"}, nil
	}
	node.scanner.poster = func(string, string) error { return nil }
	items, err := node.scanner.Scan()
	if err != nil || len(items) != 1 {
		t.Fatalf("scan=%d err=%v", len(items), err)
	}
	request := httptest.NewRequest(http.MethodGet, "/internal/media/"+items[0].MediaKey, nil)
	request.Header.Set("X-Relay-Token", "relay-token")
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	node.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" || !strings.Contains(response.Header().Get("Content-Range"), "2-5/10") {
		t.Fatalf("range=%d %q %q", response.Code, response.Body.String(), response.Header().Get("Content-Range"))
	}
	unauthorized := httptest.NewRecorder()
	node.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/internal/media/"+items[0].MediaKey, nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}
	put := httptest.NewRequest(http.MethodPut, "/internal/posters/"+items[0].MediaKey, nil)
	put.Header.Set("X-Relay-Token", "relay-token")
	removed := httptest.NewRecorder()
	node.Handler().ServeHTTP(removed, put)
	if removed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("poster put=%d", removed.Code)
	}
}

func TestScannerWithRealMedia(t *testing.T) {
	root := os.Getenv("TEST_MEDIA_ROOT")
	if root == "" {
		t.Skip("TEST_MEDIA_ROOT is not set")
	}
	scanner, err := NewScanner(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	items, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}
	ready := 0
	for _, item := range items {
		if item.Compatibility == "ready" && item.DurationMS > 0 && item.Width > 0 && item.PosterKey != "" {
			ready++
		}
	}
	if ready < 2 {
		t.Fatalf("ready media=%d, items=%+v", ready, items)
	}
}
