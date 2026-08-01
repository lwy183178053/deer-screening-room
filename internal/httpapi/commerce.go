package httpapi

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (a *API) commerceConfig(w http.ResponseWriter, r *http.Request) {
	notice, err := a.store.RedeemNotice(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"video_price": 1, "redeem_notice": notice})
}

func (a *API) wallet(w http.ResponseWriter, r *http.Request) {
	account, _, _, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	entries, err := a.store.ListWalletEntries(r.Context(), account.ID, 50)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"balance": account.Balance, "entries": entries})
}

func (a *API) redeem(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, "invalid_request", "请求格式不正确")
		return
	}
	normalized := strings.ToUpper(strings.TrimSpace(input.Code))
	hash := sha256.Sum256([]byte(normalized))
	credits, err := a.store.Redeem(r.Context(), account.ID, hash[:], a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	updated, _ := a.store.AccountByUserID(r.Context(), account.ID, a.now())
	writeJSON(w, 200, map[string]any{"credits": credits, "balance": updated.Balance})
}

func (a *API) unlockVideo(w http.ResponseWriter, r *http.Request) {
	account, ok := a.mutation(w, r)
	if !ok {
		return
	}
	videoID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || videoID <= 0 {
		writeError(w, 422, "invalid_id", "视频编号无效")
		return
	}
	reference, _ := randomToken(18)
	purchased, err := a.store.UnlockVideo(r.Context(), account.ID, videoID, reference, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	updated, _ := a.store.AccountByUserID(r.Context(), account.ID, a.now())
	writeJSON(w, 200, map[string]any{"purchased": purchased, "balance": updated.Balance})
}

func (a *API) createRedeemCodes(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		Credits int64 `json:"credits"`
		Count   int   `json:"count"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Credits <= 0 || input.Count < 1 || input.Count > 1000 {
		writeError(w, 422, "invalid_codes", "兑换码参数无效")
		return
	}
	codes := make([]string, input.Count)
	hashes := make([][]byte, input.Count)
	for index := range codes {
		token, _ := randomToken(12)
		code := "DEER-" + strings.ToUpper(token[:4]+"-"+token[4:8]+"-"+token[8:12]+"-"+token[12:16])
		codes[index] = code
		hash := sha256.Sum256([]byte(code))
		hashes[index] = hash[:]
	}
	now := a.now()
	if err := a.store.CreateRedeemCodes(r.Context(), account.ID, input.Credits, hashes, now); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), account.ID, "redeem_codes.created", "redeem_codes", fmt.Sprint(now.Unix()), map[string]any{"credits": input.Credits, "count": input.Count})
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=deer-codes-%d.txt", now.Unix()))
	_, _ = w.Write([]byte(redeemCodesText(codes)))
}

func redeemCodesText(codes []string) string {
	return strings.Join(codes, "\n") + "\n"
}

func (a *API) listRedeemCodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "used" && status != "unused" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_redeem_status", "兑换码状态无效")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_redeem_search", "搜索内容过长")
		return
	}
	codeHash, emailQuery := redeemCodeSearch(query)
	page, err := a.store.ListRedeemCodes(r.Context(), status, emailQuery, codeHash, requestedPage(r), 50)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func redeemCodeSearch(query string) ([]byte, string) {
	normalized := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(normalized, "DEER-") {
		hash := sha256.Sum256([]byte(normalized))
		return hash[:], ""
	}
	return nil, strings.TrimSpace(query)
}

func (a *API) getRedeemNotice(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	content, err := a.store.RedeemNotice(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (a *API) updateRedeemNotice(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, &input); err != nil || len([]rune(input.Content)) > 2000 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_redeem_notice", "兑换说明不能超过 2000 个字符")
		return
	}
	content := strings.TrimSpace(input.Content)
	if err := a.store.SetRedeemNotice(r.Context(), account.ID, content, a.now()); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), account.ID, "redeem_notice.updated", "site_setting", "redeem_notice", map[string]any{"length": len([]rune(content))})
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}
