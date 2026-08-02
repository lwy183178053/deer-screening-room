package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"deerroom/internal/store"
)

const SessionCookieName = "deer_session"

type NodeCredential struct {
	APIToken   string
	RelayToken string
	BaseURL    string
}

type Options struct {
	Store                   *store.Postgres
	CookieSecure            bool
	SessionTTL              time.Duration
	NodeAPIToken            string
	RelayToken              string
	HTTPClient              *http.Client
	Now                     func() time.Time
	PasswordHashConcurrency int
	UserStreamRPM           int
	NodeCredentials         map[string]NodeCredential
}

type API struct {
	store           *store.Postgres
	cookieSecure    bool
	sessionTTL      time.Duration
	nodeAPIToken    string
	relayToken      string
	httpClient      *http.Client
	now             func() time.Time
	captchas        *captchaManager
	authGuard       *authGuard
	passwordSlots   chan struct{}
	streamGuard     *streamGuard
	nodeCredentials map[string]NodeCredential
	nodeStates      *nodeStateStore
}

func New(options Options) http.Handler {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.SessionTTL <= 0 {
		options.SessionTTL = 7 * 24 * time.Hour
	}
	if options.HTTPClient == nil {
		options.HTTPClient = defaultNodeHTTPClient()
	}
	if options.PasswordHashConcurrency < 1 {
		options.PasswordHashConcurrency = 4
	}
	if options.UserStreamRPM < 1 {
		options.UserStreamRPM = 120
	}
	api := &API{
		store: options.Store, cookieSecure: options.CookieSecure, sessionTTL: options.SessionTTL,
		nodeAPIToken: options.NodeAPIToken,
		relayToken:   options.RelayToken,
		httpClient:   options.HTTPClient, now: options.Now,
		captchas: newCaptchaManager(options.Now), authGuard: newAuthGuard(options.Now),
		passwordSlots:   make(chan struct{}, options.PasswordHashConcurrency),
		streamGuard:     newStreamGuard(options.Now, options.UserStreamRPM),
		nodeCredentials: options.NodeCredentials,
		nodeStates:      newNodeStateStore(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", api.health)
	mux.HandleFunc("GET /api/v1/auth/captcha", api.captcha)
	mux.HandleFunc("POST /api/v1/auth/register", api.register)
	mux.HandleFunc("POST /api/v1/auth/login", api.login)
	mux.HandleFunc("GET /api/v1/auth/me", api.me)
	mux.HandleFunc("POST /api/v1/auth/logout", api.logout)
	mux.HandleFunc("GET /api/v1/commerce", api.commerceConfig)
	mux.HandleFunc("GET /api/v1/wallet", api.wallet)
	mux.HandleFunc("POST /api/v1/wallet/redeem", api.redeem)
	mux.HandleFunc("POST /api/v1/videos/{id}/unlock", api.unlockVideo)
	mux.HandleFunc("GET /api/v1/admin/redeem-codes", api.listRedeemCodes)
	mux.HandleFunc("POST /api/v1/admin/redeem-codes", api.createRedeemCodes)
	mux.HandleFunc("GET /api/v1/admin/redeem-notice", api.getRedeemNotice)
	mux.HandleFunc("PUT /api/v1/admin/redeem-notice", api.updateRedeemNotice)
	api.registerCatalogRoutes(mux)
	api.registerMediaRoutes(mux)
	return securityHeaders(mux)
}

func defaultNodeHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 15 * time.Second
	return &http.Client{Transport: transport}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "小鹿放映室"})
}

func (a *API) authenticate(w http.ResponseWriter, r *http.Request) (store.Account, string, []byte, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "authentication_required", "请先登录")
		return store.Account{}, "", nil, false
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	account, csrf, err := a.store.AccountBySession(r.Context(), hash[:], a.now())
	if err != nil || !account.Enabled {
		writeError(w, http.StatusUnauthorized, "session_invalid", "登录状态已失效")
		return store.Account{}, "", nil, false
	}
	return account, csrf, hash[:], true
}

func (a *API) mutation(w http.ResponseWriter, r *http.Request) (store.Account, bool) {
	account, csrf, _, ok := a.authenticate(w, r)
	if !ok {
		return store.Account{}, false
	}
	if csrf == "" || r.Header.Get("X-CSRF-Token") != csrf {
		writeError(w, http.StatusForbidden, "csrf_failed", "页面状态已过期，请刷新后重试")
		return store.Account{}, false
	}
	return account, true
}

func (a *API) adminMutation(w http.ResponseWriter, r *http.Request) (store.Account, bool) {
	account, ok := a.mutation(w, r)
	if ok && !account.IsAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "需要管理员权限")
		return store.Account{}, false
	}
	return account, ok
}

func (a *API) adminRead(w http.ResponseWriter, r *http.Request) (store.Account, bool) {
	account, _, _, ok := a.authenticate(w, r)
	if ok && !account.IsAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "需要管理员权限")
		return store.Account{}, false
	}
	return account, ok
}

func randomToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, output any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "资源不存在")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "资源状态冲突")
	case errors.Is(err, store.ErrInsufficient):
		writeError(w, http.StatusUnprocessableEntity, "insufficient_credits", "鹿币余额不足")
	case errors.Is(err, store.ErrUnavailable):
		writeError(w, http.StatusConflict, "video_unavailable", "视频暂不可用")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "playback_forbidden", "尚未获得观看权限")
	case errors.Is(err, store.ErrCodeUsed):
		writeError(w, http.StatusConflict, "redeem_code_used", "兑换码已被使用")
	case errors.Is(err, store.ErrCodeInvalid):
		writeError(w, http.StatusUnprocessableEntity, "redeem_code_invalid", "兑换码无效")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
	}
}
