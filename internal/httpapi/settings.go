package httpapi

import (
	"fmt"
	"net/http"
)

const maxUserStreamMbps = int64(1000)

func (a *API) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	bps := a.userStreamBPS.Load()
	writeJSON(w, http.StatusOK, map[string]int64{
		"user_stream_bps":  bps,
		"user_stream_mbps": bps / 1_000_000,
	})
}

func (a *API) updateAdminSettings(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		UserStreamMbps int64 `json:"user_stream_mbps"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.UserStreamMbps < 1 || input.UserStreamMbps > maxUserStreamMbps {
		writeError(w, http.StatusUnprocessableEntity, "invalid_settings", fmt.Sprintf("用户播放速率需为 1 到 %d Mbps", maxUserStreamMbps))
		return
	}
	bps := input.UserStreamMbps * 1_000_000
	if err := a.store.SetUserStreamBPS(r.Context(), account.ID, bps, a.now()); err != nil {
		storeError(w, err)
		return
	}
	a.userStreamBPS.Store(bps)
	_ = a.store.AddAudit(r.Context(), account.ID, "stream_settings.updated", "site_setting", "user_stream_bps", map[string]any{"mbps": input.UserStreamMbps})
	writeJSON(w, http.StatusOK, map[string]int64{
		"user_stream_bps":  bps,
		"user_stream_mbps": input.UserStreamMbps,
	})
}
