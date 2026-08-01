package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"deerroom/internal/media"
	"deerroom/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pion/webrtc/v4"
)

func TestP2PPlaybackIntegration(t *testing.T) {
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
	var offerCalls, closeCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node-token" {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.URL.Path {
		case "/internal/webrtc/offer":
			var offer media.WebRTCOffer
			if json.NewDecoder(r.Body).Decode(&offer) != nil || offer.MediaKey != "media-key" || offer.AllowDirect || len(offer.ICEServers) != 1 {
				http.Error(w, "invalid offer", http.StatusUnprocessableEntity)
				return
			}
			offerCalls.Add(1)
			writeJSON(w, http.StatusOK, media.WebRTCAnswer{SessionID: offer.SessionID, SDP: "node-answer", Type: "answer"})
		case "/internal/webrtc/close":
			closeCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
			return
		}
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
	handler := New(Options{Store: db, NodeAPIToken: "node-token", TurnURLs: []string{"turn:turn.example.test:3478"}, TurnSecret: "turn-secret", TurnTTL: time.Hour, Now: func() time.Time { return now }})
	create := httptest.NewRequest(http.MethodPost, "/api/v1/videos/1/p2p/session", bytes.NewReader([]byte(`{}`)))
	create.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	create.Header.Set("X-CSRF-Token", "csrf")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != 201 {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var body struct {
		SessionID  string             `json:"session_id"`
		ICEServers []webrtc.ICEServer `json:"ice_servers"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SessionID == "" || len(body.ICEServers) != 1 {
		t.Fatalf("session=%+v", body)
	}
	secondCreate := httptest.NewRequest(http.MethodPost, "/api/v1/videos/1/p2p/session", bytes.NewReader([]byte(`{}`)))
	secondCreate.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	secondCreate.Header.Set("X-CSRF-Token", "csrf")
	secondCreated := httptest.NewRecorder()
	handler.ServeHTTP(secondCreated, secondCreate)
	if secondCreated.Code != http.StatusCreated || closeCalls.Load() != 1 {
		t.Fatalf("replacement=%d close calls=%d body=%s", secondCreated.Code, closeCalls.Load(), secondCreated.Body.String())
	}
	if err := json.Unmarshal(secondCreated.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	offer := httptest.NewRequest(http.MethodPost, "/api/v1/p2p/"+body.SessionID+"/offer", bytes.NewBufferString(`{"sdp":"browser-offer","type":"offer"}`))
	offer.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	offer.Header.Set("X-CSRF-Token", "csrf")
	answered := httptest.NewRecorder()
	handler.ServeHTTP(answered, offer)
	if answered.Code != http.StatusOK || !bytes.Contains(answered.Body.Bytes(), []byte("node-answer")) || offerCalls.Load() != 1 {
		t.Fatalf("offer=%d calls=%d body=%s", answered.Code, offerCalls.Load(), answered.Body.String())
	}
	closeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/p2p/"+body.SessionID, nil)
	closeRequest.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	closeRequest.Header.Set("X-CSRF-Token", "csrf")
	closed := httptest.NewRecorder()
	handler.ServeHTTP(closed, closeRequest)
	if closed.Code != http.StatusNoContent || closeCalls.Load() != 2 {
		t.Fatalf("close=%d calls=%d", closed.Code, closeCalls.Load())
	}
	for _, removed := range []struct{ method, path string }{{http.MethodPost, "/api/v1/videos/1/playback"}, {http.MethodGet, "/api/v1/playback/old/stream"}} {
		request := httptest.NewRequest(removed.method, removed.path, bytes.NewReader([]byte(`{}`)))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("removed %s %s=%d", removed.method, removed.path, response.Code)
		}
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
	handler := New(Options{Store: db, NodeAPIToken: "node-token", Now: func() time.Time { return now }})

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
