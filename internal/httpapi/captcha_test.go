package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRenderCaptchaPNG(t *testing.T) {
	body, err := renderCaptcha("A2B3C")
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if image.Bounds().Dx() != 170 || image.Bounds().Dy() != 56 {
		t.Fatalf("captcha size=%v", image.Bounds())
	}
}

func TestCaptchaIsBoundExpiresAndIsOneTime(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	manager := newCaptchaManager(func() time.Time { return now })
	manager.challenges["valid"] = captchaChallenge{digest: sha256.Sum256([]byte("A2B3C")), clientIP: "127.0.0.1", purpose: "register", expiresAt: now.Add(captchaLifetime)}
	if manager.verify("valid", "A2B3C", "127.0.0.2", "register") {
		t.Fatal("captcha accepted for another client")
	}
	if !manager.verify("valid", "a2b3c", "127.0.0.1", "register") {
		t.Fatal("valid captcha rejected")
	}
	if manager.verify("valid", "A2B3C", "127.0.0.1", "register") {
		t.Fatal("captcha reused")
	}
	manager.challenges["expired"] = captchaChallenge{digest: sha256.Sum256([]byte("D4E5F")), clientIP: "127.0.0.1", purpose: "login", expiresAt: now.Add(time.Second)}
	now = now.Add(2 * time.Second)
	if manager.verify("expired", "D4E5F", "127.0.0.1", "login") {
		t.Fatal("expired captcha accepted")
	}
}

func TestCaptchaExpiresAfterFiveFailures(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	manager := newCaptchaManager(func() time.Time { return now })
	manager.challenges["limited"] = captchaChallenge{digest: sha256.Sum256([]byte("A2B3C")), clientIP: "127.0.0.1", purpose: "register", expiresAt: now.Add(captchaLifetime)}
	for index := 0; index < captchaMaxTries; index++ {
		if manager.verify("limited", "WRONG", "127.0.0.1", "register") {
			t.Fatal("wrong captcha accepted")
		}
	}
	if _, ok := manager.challenges["limited"]; ok {
		t.Fatal("captcha retained after maximum failures")
	}
}

func TestAuthGuardCleansExpiredEntries(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	guard := newAuthGuard(func() time.Time { return now })
	guard.register["old"] = windowEntry{count: 1, reset: now.Add(-time.Second)}
	guard.login["old"] = windowEntry{count: 1, reset: now.Add(-time.Second)}
	guard.captchaIssuing["old"] = windowEntry{count: 1, reset: now.Add(-time.Second)}
	guard.loginCaptcha["old"] = loginEntry{requiredUntil: now.Add(-time.Second)}
	guard.allowRegister("new")
	if _, ok := guard.register["old"]; ok {
		t.Fatal("expired register entry retained")
	}
	if _, ok := guard.login["old"]; ok {
		t.Fatal("expired login entry retained")
	}
	if _, ok := guard.captchaIssuing["old"]; ok {
		t.Fatal("expired captcha entry retained")
	}
	if _, ok := guard.loginCaptcha["old"]; ok {
		t.Fatal("expired login captcha entry retained")
	}
}

func TestRegisterRequiresCaptcha(t *testing.T) {
	handler := New(Options{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"12345678"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Error struct {
			Code            string `json:"code"`
			CaptchaRequired bool   `json:"captcha_required"`
		} `json:"error"`
	}
	if json.Unmarshal(response.Body.Bytes(), &body) != nil || body.Error.Code != "captcha_required" || !body.Error.CaptchaRequired {
		t.Fatalf("body=%s", response.Body.String())
	}
}
