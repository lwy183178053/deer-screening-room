package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"deerroom/internal/provisioning"
	"deerroom/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPlaybackProxyIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_HTTP_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_HTTP_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	pool.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relay-Token") != "relay-token" {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.Header.Get("Range") == "bytes=2-5" {
			w.Header().Set("Content-Range", "bytes 2-5/10")
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(206)
			_, _ = w.Write([]byte("2345"))
			return
		}
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer upstream.Close()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	user, err := db.CreateUser(ctx, "viewer@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	token := "session-token"
	tokenHash := sha256.Sum256([]byte(token))
	if err := db.CreateSession(ctx, tokenHash[:], user.ID, "csrf", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{4}, 32)
	apiSealed, _ := provisioning.Seal(key, "node-token")
	relaySealed, _ := provisioning.Seal(key, "relay-token")
	if _, err := db.CreateProvisionedNode(ctx, "node", upstream.URL, "10.77.0.2", "public", "private", apiSealed, relaySealed, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SyncMedia(ctx, "node", upstream.URL, 100, 90, []store.MediaItem{{MediaKey: "media-key", Studio: "工作室", Title: "作品", DurationMS: 1000, SizeBytes: 10, VideoCodec: "h264", AudioCodec: "aac", Compatibility: "ready"}}, now); err != nil {
		t.Fatal(err)
	}
	videos, err := db.ListVideos(ctx, user.ID, "", 0, false, now)
	if err != nil || len(videos) != 1 {
		t.Fatalf("videos=%d err=%v", len(videos), err)
	}
	if _, _, err := db.AdjustCredits(ctx, user.ID, user.ID, 10, "", "HTTP-CREDITS", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UnlockVideo(ctx, user.ID, videos[0].ID, "HTTP-UNLOCK", now); err != nil {
		t.Fatal(err)
	}
	handler := New(Options{Store: db, NodeSecretsKey: key, Now: func() time.Time { return now }})
	create := httptest.NewRequest(http.MethodPost, "/api/v1/videos/1/playback", bytes.NewReader([]byte(`{}`)))
	create.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	create.Header.Set("X-CSRF-Token", "csrf")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != 201 {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	stream := httptest.NewRequest(http.MethodGet, "/api/v1/playback/"+body.ID+"/stream", nil)
	stream.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	stream.Header.Set("Range", "bytes=2-5")
	streamed := httptest.NewRecorder()
	handler.ServeHTTP(streamed, stream)
	if streamed.Code != 206 || streamed.Body.String() != "2345" {
		t.Fatalf("stream=%d %q", streamed.Code, streamed.Body.String())
	}
}

func TestAdminUserCreditsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_HTTP_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_HTTP_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	admin, err := db.CreateUser(ctx, "admin@example.com", "hash", true)
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := db.CreateUser(ctx, "viewer@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	token := "admin-session"
	tokenHash := sha256.Sum256([]byte(token))
	if err := db.CreateSession(ctx, tokenHash[:], admin.ID, "csrf", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	handler := New(Options{Store: db, NodeSecretsKey: bytes.Repeat([]byte{4}, 32), Now: func() time.Time { return now }})

	search := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?q=VIEWER", nil)
	search.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	searched := httptest.NewRecorder()
	handler.ServeHTTP(searched, search)
	var page store.UserPage
	if searched.Code != 200 || json.Unmarshal(searched.Body.Bytes(), &page) != nil || page.Total != 1 || len(page.Users) != 1 || page.Users[0].ID != viewer.ID {
		t.Fatalf("search=%d total=%d users=%d body=%s", searched.Code, page.Total, len(page.Users), searched.Body.String())
	}

	adjust := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/2/credits", bytes.NewBufferString(body))
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", "csrf")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	first := adjust(`{"delta":10,"request_id":"adjust-request-1"}`)
	duplicate := adjust(`{"delta":10,"request_id":"adjust-request-1"}`)
	insufficient := adjust(`{"delta":-11,"request_id":"adjust-request-2"}`)
	var firstBody, duplicateBody struct {
		Account  store.Account `json:"account"`
		Adjusted bool          `json:"adjusted"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &firstBody)
	_ = json.Unmarshal(duplicate.Body.Bytes(), &duplicateBody)
	if first.Code != 200 || !firstBody.Adjusted || firstBody.Account.Balance != 10 || duplicate.Code != 200 || duplicateBody.Adjusted || duplicateBody.Account.Balance != 10 || insufficient.Code != 422 {
		t.Fatalf("adjust first=%d/%v/%d duplicate=%d/%v/%d insufficient=%d", first.Code, firstBody.Adjusted, firstBody.Account.Balance, duplicate.Code, duplicateBody.Adjusted, duplicateBody.Account.Balance, insufficient.Code)
	}
	entries, err := db.ListWalletEntries(ctx, viewer.ID, 10)
	if err != nil || len(entries) != 1 || entries[0].Description != "管理员增加鹿币" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action='wallet.admin_adjusted'`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audits=%d err=%v", auditCount, err)
	}

	removedRoutes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/videos"},
		{http.MethodGet, "/api/v1/admin/orders"},
		{http.MethodPost, "/api/v1/payments/zpay/orders"},
		{http.MethodGet, "/api/v1/admin/settings"},
		{http.MethodPut, "/api/v1/admin/settings"},
	}
	for _, route := range removedRoutes {
		request := httptest.NewRequest(route.method, route.path, nil)
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("removed route %s %s=%d", route.method, route.path, response.Code)
		}
	}
}
