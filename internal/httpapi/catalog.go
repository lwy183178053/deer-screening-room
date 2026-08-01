package httpapi

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"deerroom/internal/password"
	"deerroom/internal/store"
)

func (a *API) registerCatalogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/studios", a.listStudios)
	mux.HandleFunc("GET /api/v1/videos", a.listVideos)
	mux.HandleFunc("GET /api/v1/videos/{id}", a.getVideo)
	mux.HandleFunc("GET /api/v1/videos/{id}/poster", a.getPoster)
	mux.HandleFunc("GET /api/v1/library", a.library)
	mux.HandleFunc("GET /api/v1/admin/users", a.adminUsers)
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", a.updateAdminUser)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/reset-password", a.resetAdminPassword)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/credits", a.adjustAdminCredits)
	mux.HandleFunc("GET /api/v1/admin/nodes", a.adminNodes)
	mux.HandleFunc("POST /api/v1/admin/nodes/{id}/rescan", a.adminRescan)
}

func (a *API) listStudios(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListStudios(r.Context(), a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"studios": items})
}

func (a *API) listVideos(w http.ResponseWriter, r *http.Request) {
	userID := a.optionalUserID(r)
	studioID, _ := strconv.ParseInt(r.URL.Query().Get("studio_id"), 10, 64)
	seed, ok := requestedSeed(w, r)
	if !ok {
		return
	}
	page := requestedPage(r)
	result, err := a.store.ListVideosPage(r.Context(), userID, strings.TrimSpace(r.URL.Query().Get("q")), studioID, false, a.now(), page, 50, seed)
	if err != nil {
		storeError(w, err)
		return
	}
	decorateVideos(result.Videos)
	writeJSON(w, 200, result)
}

func (a *API) getVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	item, err := a.store.VideoByID(r.Context(), a.optionalUserID(r), id, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	decorateVideo(&item)
	writeJSON(w, 200, map[string]any{"video": item})
}

func (a *API) library(w http.ResponseWriter, r *http.Request) {
	account, _, _, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.store.ListVideosPage(r.Context(), account.ID, "", 0, true, a.now(), requestedPage(r), 50, 0)
	if err != nil {
		storeError(w, err)
		return
	}
	decorateVideos(result.Videos)
	writeJSON(w, 200, result)
}

func (a *API) getPoster(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	video, err := a.store.VideoByID(r.Context(), 0, id, a.now())
	if err != nil || video.PosterKey == "" {
		http.NotFound(w, r)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, video.NodeURL+"/internal/posters/"+video.PosterKey, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	relayToken, ok := a.relayTokenFor(video.NodeName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	request.Header.Set("X-Relay-Token", relayToken)
	response, err := a.httpClient.Do(request)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, response.Body)
}

func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	result, err := a.store.ListUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), requestedPage(r), 50, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) updateAdminUser(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Enabled == nil {
		writeError(w, 422, "invalid_user", "用户状态无效")
		return
	}
	if id == account.ID && !*input.Enabled {
		writeError(w, 409, "self_disable", "不能停用当前管理员")
		return
	}
	if err := a.store.SetUserEnabled(r.Context(), id, *input.Enabled); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), account.ID, "user.enabled_changed", "user", fmt.Sprint(id), input)
	w.WriteHeader(204)
}
func (a *API) resetAdminPassword(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil || !validPassword(input.Password) {
		writeError(w, 422, "invalid_password", "密码长度需为8到64个字符")
		return
	}
	hash, err := password.Hash(input.Password)
	if err != nil {
		storeError(w, err)
		return
	}
	if err := a.store.ResetUserPassword(r.Context(), id, hash); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), account.ID, "user.password_reset", "user", fmt.Sprint(id), map[string]any{})
	w.WriteHeader(204)
}
func (a *API) adjustAdminCredits(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Delta     int64  `json:"delta"`
		Reason    string `json:"reason"`
		RequestID string `json:"request_id"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, "invalid_request", "请求格式不正确")
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.Delta == 0 || input.Delta < -100000 || input.Delta > 100000 || len([]rune(input.Reason)) > 200 || len(input.RequestID) < 8 || len(input.RequestID) > 100 {
		writeError(w, 422, "invalid_adjustment", "鹿币调整参数无效")
		return
	}
	account, adjusted, err := a.store.AdjustCredits(r.Context(), actor.ID, id, input.Delta, input.Reason, input.RequestID, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"account": account, "adjusted": adjusted})
}
func (a *API) adminNodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	items, err := a.store.ListNodes(r.Context(), a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"nodes": a.nodeStates.response(items)})
}
func (a *API) adminRescan(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminMutation(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	node, err := a.store.NodeByID(r.Context(), id, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, node.BaseURL+"/internal/rescan", nil)
	if err != nil {
		storeError(w, err)
		return
	}
	relayToken, ok := a.relayTokenFor(node.Name)
	if !ok {
		writeError(w, http.StatusBadGateway, "node_configuration_invalid", "节点配置无效")
		return
	}
	request.Header.Set("X-Relay-Token", relayToken)
	response, err := a.httpClient.Do(request)
	if err != nil {
		writeError(w, 502, "node_unavailable", "无法连接媒体节点")
		return
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		writeError(w, 502, "scan_failed", "节点扫描失败")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "scanning"})
}

func (a *API) optionalUserID(r *http.Request) int64 {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return 0
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	account, _, err := a.store.AccountBySession(r.Context(), hash[:], a.now())
	if err != nil || !account.Enabled {
		return 0
	}
	return account.ID
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 422, "invalid_id", "资源编号无效")
		return 0, false
	}
	return id, true
}

func requestedPage(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		return 1
	}
	return min(page, 100000)
}

func requestedSeed(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("seed"))
	if raw == "" {
		return 0, true
	}
	seed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seed <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_seed", "随机种子无效")
		return 0, false
	}
	return seed, true
}

func decorateVideos(items []store.Video) {
	for index := range items {
		decorateVideo(&items[index])
	}
}
func decorateVideo(item *store.Video) {
	if item.PosterKey != "" {
		item.PosterURL = fmt.Sprintf("/api/v1/videos/%d/poster", item.ID)
	}
}
