package httpapi

import (
	"crypto/sha256"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"deerroom/internal/password"
	"deerroom/internal/store"
)

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captcha_id"`
		CaptchaAnswer string `json:"captcha_answer"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, "invalid_request", "请求格式不正确")
		return
	}
	ip := clientIP(r)
	if ok, retry := a.authGuard.allowRegister(ip); !ok {
		writeRateLimited(w, retry)
		return
	}
	if input.CaptchaID == "" || input.CaptchaAnswer == "" {
		writeCaptchaError(w, "captcha_required", "请输入验证码")
		return
	}
	if !a.captchas.verify(input.CaptchaID, input.CaptchaAnswer, ip, "register") {
		writeCaptchaError(w, "captcha_invalid", "验证码错误或已过期")
		return
	}
	email, ok := normalizeEmail(input.Email)
	if !ok {
		writeError(w, 422, "invalid_email", "邮箱格式无效")
		return
	}
	if !validPassword(input.Password) {
		writeError(w, 422, "invalid_password", "密码长度需为8到64个字符")
		return
	}
	if !a.acquirePasswordSlot(w) {
		return
	}
	hash, err := password.Hash(input.Password)
	a.releasePasswordSlot()
	if err != nil {
		storeError(w, err)
		return
	}
	user, err := a.store.CreateUser(r.Context(), email, hash, false)
	if err != nil {
		storeError(w, err)
		return
	}
	a.createLogin(w, r, user)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captcha_id"`
		CaptchaAnswer string `json:"captcha_answer"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, "invalid_request", "请求格式不正确")
		return
	}
	ip := clientIP(r)
	if ok, retry := a.authGuard.allowLogin(ip); !ok {
		writeRateLimited(w, retry)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	guardKey := ip + "\x00" + email
	if a.authGuard.requireLoginCaptcha(guardKey) {
		if input.CaptchaID == "" || input.CaptchaAnswer == "" {
			writeCaptchaError(w, "captcha_required", "请输入验证码")
			return
		}
		if !a.captchas.verify(input.CaptchaID, input.CaptchaAnswer, ip, "login") {
			writeCaptchaError(w, "captcha_invalid", "验证码错误或已过期")
			return
		}
	}
	user, err := a.store.UserByEmail(r.Context(), email)
	if err != nil || !user.Enabled {
		a.authGuard.markLoginFailure(guardKey)
		writeLoginFailure(w)
		return
	}
	if !a.acquirePasswordSlot(w) {
		return
	}
	ok, err := password.Verify(user.PasswordHash, input.Password)
	a.releasePasswordSlot()
	if err != nil || !ok {
		a.authGuard.markLoginFailure(guardKey)
		writeLoginFailure(w)
		return
	}
	a.authGuard.clearLoginFailure(guardKey)
	a.createLogin(w, r, user.User)
}

func (a *API) acquirePasswordSlot(w http.ResponseWriter) bool {
	select {
	case a.passwordSlots <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "authentication_busy", "登录请求较多，请稍后重试")
		return false
	}
}

func (a *API) releasePasswordSlot() { <-a.passwordSlots }

func writeCaptchaError(w http.ResponseWriter, code, message string) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"code": code, "message": message, "captcha_required": true}})
}

func writeLoginFailure(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "invalid_credentials", "message": "邮箱或密码错误", "captcha_required": true}})
}

func (a *API) createLogin(w http.ResponseWriter, r *http.Request, user store.User) {
	token, err := randomToken(32)
	if err != nil {
		storeError(w, err)
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		storeError(w, err)
		return
	}
	hash := sha256.Sum256([]byte(token))
	if err := a.store.CreateSession(r.Context(), hash[:], user.ID, csrf, a.now().Add(a.sessionTTL)); err != nil {
		storeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: int(a.sessionTTL.Seconds())})
	account, err := a.store.AccountByUserID(r.Context(), user.ID, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account": account, "csrf_token": csrf})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	account, csrf, _, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"account": account, "csrf_token": csrf})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	_, csrf, hash, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if r.Header.Get("X-CSRF-Token") != csrf {
		writeError(w, 403, "csrf_failed", "页面状态已过期")
		return
	}
	_ = a.store.DeleteSession(r.Context(), hash)
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func normalizeEmail(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	address, err := mail.ParseAddress(raw)
	if err != nil || address.Address != raw || strings.ContainsAny(raw, "<>\r\n") {
		return "", false
	}
	return strings.ToLower(raw), true
}

func validPassword(value string) bool {
	length := utf8.RuneCountInString(value)
	return length >= 8 && length <= 64
}
