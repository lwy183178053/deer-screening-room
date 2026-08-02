package httpapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"deerroom/internal/media"
	"deerroom/internal/store"
	"github.com/pion/webrtc/v4"
)

func (a *API) registerMediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/internal/media/sync", a.syncMedia)
	mux.HandleFunc("POST /api/v1/internal/media/heartbeat", a.mediaHeartbeat)
	mux.HandleFunc("POST /api/v1/videos/{id}/p2p/session", a.createP2PSession)
	mux.HandleFunc("POST /api/v1/p2p/{id}/offer", a.offerP2P)
	mux.HandleFunc("DELETE /api/v1/p2p/{id}", a.closeP2P)
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
	for i, item := range input.Items {
		items[i] = store.MediaItem{MediaKey: item.MediaKey, Studio: item.Studio, Title: item.Title, PosterKey: item.PosterKey, DurationMS: item.DurationMS, SizeBytes: item.SizeBytes, BitRate: item.BitRate, Width: item.Width, Height: item.Height, VideoCodec: item.VideoCodec, AudioCodec: item.AudioCodec, Compatibility: item.Compatibility}
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

func (a *API) createP2PSession(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	if allowed, retry := a.streamGuard.allow(account.ID); !allowed {
		writeRateLimited(w, retry)
		return
	}
	videoID, ok := pathID(w, r)
	if !ok {
		return
	}
	id, err := randomToken(24)
	if err != nil {
		storeError(w, err)
		return
	}
	expires := a.now().Add(6 * time.Hour)
	revoked, err := a.store.CreatePlayback(r.Context(), id, account.ID, videoID, expires, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	for _, target := range revoked {
		a.notifyNodeClose(r, target.NodeName, target.NodeURL, target.SessionID)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session_id": id, "expires_at": expires, "p2p_enabled": a.p2pEnabled.Load(), "ice_servers": a.turnServers()})
}

func (a *API) offerP2P(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	video, err := a.store.PlaybackVideo(r.Context(), r.PathValue("id"), account.ID, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	var input struct {
		SDP  string `json:"sdp"`
		Type string `json:"type"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.SDP == "" {
		writeError(w, 422, "invalid_offer", "WebRTC offer 无效")
		return
	}
	body, err := a.nodeOfferPayload(r.PathValue("id"), video.MediaKey, input.SDP, input.Type)
	if err != nil {
		storeError(w, err)
		return
	}
	response, err := a.nodeRequest(r, video.NodeName, video.NodeURL, http.MethodPost, "/internal/webrtc/offer", body)
	if err != nil {
		writeError(w, 502, "node_unavailable", "媒体节点无法建立 P2P 播放")
		return
	}
	defer response.Body.Close()
	copyStatusJSON(w, response)
}

func (a *API) closeP2P(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	video, err := a.store.PlaybackVideo(r.Context(), r.PathValue("id"), account.ID, a.now())
	if err == nil {
		a.notifyNodeClose(r, video.NodeName, video.NodeURL, r.PathValue("id"))
	}
	if err := a.store.RevokePlayback(r.Context(), r.PathValue("id"), account.ID, a.now()); err != nil && err != store.ErrNotFound {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) notifyNodeClose(r *http.Request, nodeName, nodeURL, sessionID string) {
	body, _ := json.Marshal(map[string]string{"session_id": sessionID})
	if response, err := a.nodeRequest(r, nodeName, nodeURL, http.MethodPost, "/internal/webrtc/close", body); err == nil {
		response.Body.Close()
	}
}

func (a *API) nodeRequest(r *http.Request, nodeName, baseURL, method, path string, body []byte) (*http.Response, error) {
	request, err := http.NewRequestWithContext(r.Context(), method, strings.TrimRight(baseURL, "/")+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+a.nodeTokenFor(nodeName))
	return a.httpClient.Do(request)
}

func (a *API) nodeTokenFor(nodeName string) string {
	if credential, ok := a.nodeCredentials[nodeName]; ok {
		return credential.APIToken
	}
	return a.nodeAPIToken
}

func (a *API) nodeOfferPayload(sessionID, mediaKey, sdp, sdpType string) ([]byte, error) {
	return json.Marshal(media.WebRTCOffer{
		SessionID: sessionID, MediaKey: mediaKey, SDP: sdp, Type: sdpType,
		ICEServers: a.turnServers(), AllowDirect: a.p2pEnabled.Load(),
	})
}

func (a *API) turnServers() []webrtc.ICEServer {
	if len(a.turnURLs) == 0 || a.turnSecret == "" {
		return nil
	}
	expires := a.now().Add(a.turnTTL).Unix()
	username := fmt.Sprintf("%d:%s", expires, "deerroom")
	h := hmac.New(sha1.New, []byte(a.turnSecret))
	_, _ = h.Write([]byte(username))
	servers := make([]webrtc.ICEServer, 0, 2)
	if a.p2pEnabled.Load() && len(a.stunURLs) > 0 {
		servers = append(servers, webrtc.ICEServer{URLs: a.stunURLs})
	}
	servers = append(servers, webrtc.ICEServer{URLs: a.turnURLs, Username: username, Credential: base64.StdEncoding.EncodeToString(h.Sum(nil))})
	return servers
}

func (a *API) nodeAuthorized(r *http.Request, nodeName, baseURL string) bool {
	received := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if credential, ok := a.nodeCredentials[nodeName]; len(a.nodeCredentials) > 0 {
		return ok && baseURL == credential.BaseURL && constantToken(received, credential.APIToken)
	}
	return constantToken(received, a.nodeAPIToken)
}
func constantToken(received, expected string) bool {
	return received != "" && len(received) == len(expected) && subtle.ConstantTimeCompare([]byte(received), []byte(expected)) == 1
}
func decodeLargeJSON(w http.ResponseWriter, r *http.Request, output any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}
func copyStatusJSON(w http.ResponseWriter, response *http.Response) {
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
