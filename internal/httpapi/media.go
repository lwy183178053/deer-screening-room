package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"deerroom/internal/media"
	"deerroom/internal/store"
)

var forwardedRequestHeaders = []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"}
var forwardedResponseHeaders = []string{"Accept-Ranges", "Content-Length", "Content-Range", "Content-Type", "ETag", "Last-Modified"}

func (a *API) registerMediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/internal/media/sync", a.syncMedia)
	mux.HandleFunc("POST /api/v1/internal/media/heartbeat", a.mediaHeartbeat)
	mux.HandleFunc("POST /api/v1/videos/{id}/playback", a.createPlayback)
	mux.HandleFunc("GET /api/v1/playback/{id}/stream", a.streamPlayback)
	mux.HandleFunc("HEAD /api/v1/playback/{id}/stream", a.streamPlayback)
}

func (a *API) syncMedia(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NodeName          string       `json:"node_name"`
		BaseURL           string       `json:"base_url"`
		TotalBytes        int64        `json:"total_bytes"`
		AvailableBytes    int64        `json:"available_bytes"`
		Items             []media.Item `json:"items"`
		InventoryRevision string       `json:"inventory_revision"`
		ScanStatus        string       `json:"scan_status"`
		LastScanAt        time.Time    `json:"last_scan_at"`
		ScanError         string       `json:"scan_error"`
	}
	if err := decodeLargeJSON(w, r, &input); err != nil || input.NodeName == "" || input.BaseURL == "" {
		writeError(w, 422, "invalid_inventory", "媒体清单无效")
		return
	}
	input.BaseURL = strings.TrimRight(input.BaseURL, "/")
	if !a.nodeAuthorized(r, input.NodeName, input.BaseURL) {
		writeError(w, 401, "node_token_invalid", "节点凭据无效")
		return
	}
	items := make([]store.MediaItem, len(input.Items))
	for index, item := range input.Items {
		items[index] = store.MediaItem{MediaKey: item.MediaKey, Studio: item.Studio, Title: item.Title, PosterKey: item.PosterKey, DurationMS: item.DurationMS, SizeBytes: item.SizeBytes, BitRate: item.BitRate, Width: item.Width, Height: item.Height, VideoCodec: item.VideoCodec, AudioCodec: item.AudioCodec, Compatibility: item.Compatibility}
	}
	nodeID, err := a.store.SyncMedia(r.Context(), input.NodeName, input.BaseURL, input.TotalBytes, input.AvailableBytes, items, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	a.nodeStates.update(input.NodeName, input.ScanStatus, input.LastScanAt, input.ScanError, input.InventoryRevision)
	writeJSON(w, 200, map[string]any{"node_id": nodeID, "videos": len(items)})
}
func (a *API) mediaHeartbeat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NodeName          string    `json:"node_name"`
		BaseURL           string    `json:"base_url"`
		TotalBytes        int64     `json:"total_bytes"`
		AvailableBytes    int64     `json:"available_bytes"`
		InventoryRevision string    `json:"inventory_revision"`
		ScanStatus        string    `json:"scan_status"`
		LastScanAt        time.Time `json:"last_scan_at"`
		ScanError         string    `json:"scan_error"`
	}
	if err := decodeLargeJSON(w, r, &input); err != nil || input.NodeName == "" || input.BaseURL == "" {
		writeError(w, 422, "invalid_heartbeat", "节点状态无效")
		return
	}
	input.BaseURL = strings.TrimRight(input.BaseURL, "/")
	if !a.nodeAuthorized(r, input.NodeName, input.BaseURL) {
		writeError(w, 401, "node_token_invalid", "节点凭据无效")
		return
	}
	if err := a.store.HeartbeatNode(r.Context(), input.NodeName, input.BaseURL, input.TotalBytes, input.AvailableBytes, a.now()); err != nil {
		storeError(w, err)
		return
	}
	a.nodeStates.update(input.NodeName, input.ScanStatus, input.LastScanAt, input.ScanError, input.InventoryRevision)
	w.WriteHeader(204)
}

func (a *API) createPlayback(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	videoID, ok := pathID(w, r)
	if !ok {
		return
	}
	id, _ := randomToken(24)
	if err := a.store.CreatePlayback(r.Context(), id, account.ID, videoID, a.now().Add(6*time.Hour), a.now()); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "stream_url": "/api/v1/playback/" + id + "/stream"})
}

func (a *API) streamPlayback(w http.ResponseWriter, r *http.Request) {
	account, _, _, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	allowed, retry := a.streamGuard.begin(account.ID)
	if !allowed {
		writeRateLimited(w, retry)
		return
	}
	video, err := a.store.PlaybackVideo(r.Context(), r.PathValue("id"), account.ID, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, video.NodeURL+"/internal/media/"+video.MediaKey, nil)
	if err != nil {
		storeError(w, err)
		return
	}
	relayToken, ok := a.relayTokenFor(video.NodeName)
	if !ok {
		writeError(w, http.StatusBadGateway, "node_configuration_invalid", "媒体节点配置无效")
		return
	}
	request.Header.Set("X-Relay-Token", relayToken)
	for _, name := range forwardedRequestHeaders {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		writeError(w, 502, "node_unavailable", "媒体节点不可用")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 && response.StatusCode != 206 && response.StatusCode != 304 && response.StatusCode != 416 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		writeError(w, 502, "stream_failed", "媒体节点拒绝播放")
		return
	}
	for _, name := range forwardedResponseHeaders {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	if disposition := mime.FormatMediaType("inline", map[string]string{"filename": video.Title + ".mp4"}); disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(response.StatusCode)
	if r.Method == http.MethodGet && (response.StatusCode == 200 || response.StatusCode == 206) {
		_, _ = io.Copy(w, response.Body)
	}
}

func (a *API) nodeAuthorized(r *http.Request, nodeName, baseURL string) bool {
	received := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if credential, ok := a.nodeCredentials[nodeName]; len(a.nodeCredentials) > 0 {
		return ok && baseURL == credential.BaseURL && constantToken(received, credential.APIToken)
	}
	return constantToken(received, a.nodeAPIToken)
}

func (a *API) relayTokenFor(nodeName string) (string, bool) {
	if len(a.nodeCredentials) > 0 {
		credential, ok := a.nodeCredentials[nodeName]
		return credential.RelayToken, ok && credential.RelayToken != ""
	}
	return a.relayToken, a.relayToken != ""
}

func constantToken(received, expected string) bool {
	return received != "" && len(received) == len(expected) && subtle.ConstantTimeCompare([]byte(received), []byte(expected)) == 1
}
func decodeLargeJSON(w http.ResponseWriter, r *http.Request, output any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}
