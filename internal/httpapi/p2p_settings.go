package httpapi

import "net/http"

func (a *API) getAdminP2PSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"p2p_enabled": a.p2pEnabled.Load()})
}

func (a *API) updateAdminP2PSettings(w http.ResponseWriter, r *http.Request) {
	account, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		P2PEnabled bool `json:"p2p_enabled"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_settings", "P2P 设置无效")
		return
	}
	if err := a.store.SetP2PEnabled(r.Context(), account.ID, input.P2PEnabled, a.now()); err != nil {
		storeError(w, err)
		return
	}
	a.p2pEnabled.Store(input.P2PEnabled)
	writeJSON(w, http.StatusOK, map[string]bool{"p2p_enabled": input.P2PEnabled})
}
