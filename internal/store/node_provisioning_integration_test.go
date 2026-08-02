package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeleteNodeRemovesCatalogAndEntitlements(t *testing.T) {
	databaseURL := os.Getenv("TEST_HTTP_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_HTTP_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := Open(ctx, databaseURL)
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
	if _, err := pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_node_provisioning,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	node, err := db.CreateProvisionedNode(ctx, "test-node", "http://10.77.0.2:8081", "10.77.0.2", "public", "private", "api", "relay", now)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, "viewer@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SyncMedia(ctx, node.Name, node.BaseURL, 100, 90, []MediaItem{{MediaKey: "media", Studio: "studio", Title: "title", VideoCodec: "h264", AudioCodec: "aac", Compatibility: "ready"}}, now); err != nil {
		t.Fatal(err)
	}
	videos, err := db.ListVideos(ctx, user.ID, "", 0, false, now)
	if err != nil || len(videos) != 1 {
		t.Fatalf("videos=%d err=%v", len(videos), err)
	}
	if _, _, err := db.AdjustCredits(ctx, user.ID, user.ID, 2, "", "delete-node-credit", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UnlockVideo(ctx, user.ID, videos[0].ID, "delete-node-unlock", now); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePlayback(ctx, "playback", user.ID, videos[0].ID, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteNode(ctx, node.ID); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM media_nodes`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("nodes=%d err=%v", remaining, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM videos`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("videos=%d err=%v", remaining, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM video_entitlements`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("entitlements=%d err=%v", remaining, err)
	}
}
