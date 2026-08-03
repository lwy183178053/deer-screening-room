package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWalletTransactionsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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
	if _, err := db.pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	admin, err := db.CreateUser(ctx, "admin@example.com", "hash", true)
	if err != nil {
		t.Fatal(err)
	}
	userA, err := db.CreateUser(ctx, "a@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	userB, err := db.CreateUser(ctx, "b@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	if notice, err := db.RedeemNotice(ctx); err != nil || notice != "" {
		t.Fatalf("initial redeem notice=%q err=%v", notice, err)
	}
	if err := db.SetRedeemNotice(ctx, admin.ID, "请先在卡网兑换，再输入卡密。", now); err != nil {
		t.Fatal(err)
	}
	if notice, err := db.RedeemNotice(ctx); err != nil || notice != "请先在卡网兑换，再输入卡密。" {
		t.Fatalf("saved redeem notice=%q err=%v", notice, err)
	}

	account, adjusted, err := db.AdjustCredits(ctx, admin.ID, userA.ID, 300, "", "ADJUST-1", now)
	if err != nil || !adjusted || account.Balance != 300 {
		t.Fatalf("first adjustment=%v balance=%d err=%v", adjusted, account.Balance, err)
	}
	account, adjusted, err = db.AdjustCredits(ctx, admin.ID, userA.ID, 300, "", "ADJUST-1", now.Add(time.Minute))
	if err != nil || adjusted || account.Balance != 300 {
		t.Fatalf("duplicate adjustment=%v balance=%d err=%v", adjusted, account.Balance, err)
	}
	if _, _, err := db.AdjustCredits(ctx, admin.ID, userA.ID, -301, "", "ADJUST-2", now); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("negative balance adjustment err=%v", err)
	}
	var auditCount int
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action='wallet.admin_adjusted'`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("adjustment audits=%d err=%v", auditCount, err)
	}
	userPage, err := db.ListUsers(ctx, "A@EXAMPLE", 1, 50, now)
	if err != nil || userPage.Total != 1 || userPage.AllTotal != 3 || len(userPage.Users) != 1 || userPage.Users[0].ID != userA.ID {
		t.Fatalf("user search total/all_total/len=%d/%d/%d err=%v", userPage.Total, userPage.AllTotal, len(userPage.Users), err)
	}

	code := "DEER-TEST-CODE"
	hash := sha256.Sum256([]byte(code))
	if err := db.CreateRedeemCodes(ctx, admin.ID, 8, [][]byte{hash[:]}, now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, userID := range []int64{userA.ID, userB.ID} {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); _, err := db.Redeem(ctx, id, hash[:], now); results <- err }(userID)
	}
	wg.Wait()
	close(results)
	success, used := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCodeUsed) {
			used++
		} else {
			t.Fatalf("unexpected redeem error: %v", err)
		}
	}
	if success != 1 || used != 1 {
		t.Fatalf("redeem success/used=%d/%d", success, used)
	}
	codePage, err := db.ListRedeemCodes(ctx, "all", "", nil, 1, 50)
	if err != nil || codePage.Total != 1 || codePage.Counts.All != 1 || codePage.Counts.Used != 1 || codePage.Counts.Unused != 0 || len(codePage.Codes) != 1 || !codePage.Codes[0].Used {
		t.Fatalf("redeem code page=%+v err=%v", codePage, err)
	}

	var nodeID, studioID, videoID int64
	if err := db.pool.QueryRow(ctx, `INSERT INTO media_nodes(name,base_url,online,last_seen_at) VALUES('node','http://node',true,$1) RETURNING id`, now).Scan(&nodeID); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `INSERT INTO studios(node_id,source_name,name) VALUES($1,'studio','Studio') RETURNING id`, nodeID).Scan(&studioID); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `INSERT INTO videos(node_id,studio_id,media_key,source_title,title,compatibility,published,available,last_seen_at) VALUES($1,$2,'video','Video','Video','ready',true,true,$3) RETURNING id`, nodeID, studioID, now).Scan(&videoID); err != nil {
		t.Fatal(err)
	}
	before, _ := db.AccountByUserID(ctx, userA.ID, now)
	if purchased, err := db.UnlockVideo(ctx, userA.ID, videoID, "unlock-1", now); err != nil || !purchased {
		t.Fatalf("unlock=%v err=%v", purchased, err)
	}
	if purchased, err := db.UnlockVideo(ctx, userA.ID, videoID, "unlock-2", now); err != nil || purchased {
		t.Fatalf("duplicate unlock=%v err=%v", purchased, err)
	}
	after, _ := db.AccountByUserID(ctx, userA.ID, now)
	if before.Balance-after.Balance != 1 {
		t.Fatalf("unlock charged %d", before.Balance-after.Balance)
	}
	if err := db.CreatePlayback(ctx, "play-1", userA.ID, videoID, now.Add(6*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePlayback(ctx, "play-2", userA.ID, videoID, now.Add(6*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PlaybackVideo(ctx, "play-1", userA.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old playback err=%v", err)
	}
	if video, err := db.PlaybackVideo(ctx, "play-2", userA.ID, now); err != nil || video.ID != videoID {
		t.Fatalf("new playback=%d err=%v", video.ID, err)
	}
	start := make(chan struct{})
	errors := make(chan error, 2)
	for _, playbackID := range []string{"play-concurrent-1", "play-concurrent-2"} {
		go func(id string) {
			<-start
			errors <- db.CreatePlayback(ctx, id, userA.ID, videoID, now.Add(6*time.Hour), now)
		}(playbackID)
	}
	close(start)
	for index := 0; index < 2; index++ {
		if err := <-errors; err != nil {
			t.Fatalf("concurrent playback: %v", err)
		}
	}
	var activePlaybackCount int
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM playback_sessions WHERE user_id=$1 AND revoked_at IS NULL`, userA.ID).Scan(&activePlaybackCount); err != nil || activePlaybackCount != 1 {
		t.Fatalf("active playbacks=%d err=%v", activePlaybackCount, err)
	}
}

func TestFlattenRedeemMigrationIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("redeem_migration_%d", time.Now().UnixNano())
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rootConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	rootPool, err := pgxpool.NewWithConfig(ctx, rootConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer rootPool.Close()
	if _, err := rootPool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer rootPool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	db := &Postgres{pool: pool}
	initial, err := migrationFiles.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(initial)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES('001_initial.sql')`); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,is_admin) VALUES('migration@example.com','hash',true) RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO wallets(user_id) VALUES($1)`, userID); err != nil {
		t.Fatal(err)
	}
	var batchID int64
	createdAt := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	if err := pool.QueryRow(ctx, `INSERT INTO redeem_batches(name,credits,code_count,created_by,created_at) VALUES('legacy',7,2,$1,$2) RETURNING id`, userID, createdAt).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	hashA := sha256.Sum256([]byte("DEER-MIGRATION-A"))
	hashB := sha256.Sum256([]byte("DEER-MIGRATION-B"))
	if _, err := pool.Exec(ctx, `INSERT INTO redeem_codes(batch_id,code_hash) VALUES($1,$2),($1,$3)`, batchID, hashA[:], hashB[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE redeem_codes SET redeemed_by=$1,redeemed_at=$2 WHERE code_hash=$3`, userID, createdAt, hashB[:]); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var batchTable *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('redeem_batches')`).Scan(&batchTable); err != nil {
		t.Fatal(err)
	}
	if batchTable != nil {
		t.Fatalf("legacy batch table remains: %s", *batchTable)
	}
	var migrated int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM redeem_codes WHERE credits=7 AND created_by=$1 AND created_at=$2`, userID, createdAt).Scan(&migrated); err != nil || migrated != 2 {
		t.Fatalf("migrated codes=%d err=%v", migrated, err)
	}
	var redeemed int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM redeem_codes WHERE redeemed_by=$1 AND redeemed_at=$2`, userID, createdAt).Scan(&redeemed); err != nil || redeemed != 1 {
		t.Fatalf("redeemed codes=%d err=%v", redeemed, err)
	}
}

func TestCatalogPaginationIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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
	if _, err := db.pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	items := make([]MediaItem, 0, 45)
	for index := 0; index < 45; index++ {
		studio := "Studio B"
		title := fmt.Sprintf("Beta %02d", index)
		if index < 30 {
			studio = "Studio A"
			title = fmt.Sprintf("Alpha %02d", index)
		}
		items = append(items, MediaItem{MediaKey: fmt.Sprintf("media-%02d", index), Studio: studio, Title: title, Compatibility: "ready"})
	}
	if _, err := db.SyncMedia(ctx, "node", "http://node", 0, 0, items, now); err != nil {
		t.Fatal(err)
	}

	first, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 1, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Videos) != 45 || first.Total != 45 || first.Page != 1 || first.PageSize != 50 {
		t.Fatalf("first page len/total/page/size=%d/%d/%d/%d", len(first.Videos), first.Total, first.Page, first.PageSize)
	}
	second, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 2, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Videos) != 0 || second.Total != 45 || second.Page != 2 {
		t.Fatalf("second page len/total/page=%d/%d/%d", len(second.Videos), second.Total, second.Page)
	}

	studios, err := db.ListStudios(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	var studioAID int64
	for _, studio := range studios {
		if studio.Name == "Studio A" {
			studioAID = studio.ID
		}
	}
	filtered, err := db.ListVideosPage(ctx, 0, "Alpha", studioAID, false, now, 1, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if studioAID == 0 || len(filtered.Videos) != 10 || filtered.Total != 30 {
		t.Fatalf("filtered studio/len/total=%d/%d/%d", studioAID, len(filtered.Videos), filtered.Total)
	}
	for _, video := range filtered.Videos {
		if video.StudioID != studioAID {
			t.Fatalf("filtered video studio=%d want=%d", video.StudioID, studioAID)
		}
	}

	randomFirst, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 1, 50, 3180615598)
	if err != nil {
		t.Fatal(err)
	}
	randomRepeat, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 1, 50, 3180615598)
	if err != nil {
		t.Fatal(err)
	}
	randomSecond, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 2, 50, 3180615598)
	if err != nil {
		t.Fatal(err)
	}
	differentSeed, err := db.ListVideosPage(ctx, 0, "", 0, false, now, 1, 50, 86617962)
	if err != nil {
		t.Fatal(err)
	}
	if videoIDs(randomFirst.Videos) != videoIDs(randomRepeat.Videos) {
		t.Fatal("same seed returned a different order")
	}
	if videoIDs(randomFirst.Videos) == videoIDs(differentSeed.Videos) {
		t.Fatal("different seeds returned the same order")
	}
	seen := make(map[int64]bool, 45)
	for _, video := range append(randomFirst.Videos, randomSecond.Videos...) {
		if seen[video.ID] {
			t.Fatalf("random pagination repeated video %d", video.ID)
		}
		seen[video.ID] = true
	}
	if len(seen) != 45 {
		t.Fatalf("random pagination returned %d unique videos, want 45", len(seen))
	}
}

func TestCatalogHidesOnlyOfflineNode(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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
	if _, err := db.pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	itemA := MediaItem{MediaKey: "video-a", Studio: "工作室甲", Title: "作品甲", SizeBytes: 10, VideoCodec: "h264", AudioCodec: "aac", Compatibility: "ready"}
	itemB := MediaItem{MediaKey: "video-b", Studio: "工作室乙", Title: "作品乙", SizeBytes: 20, VideoCodec: "h264", AudioCodec: "aac", Compatibility: "ready"}
	if _, err := db.SyncMedia(ctx, "node-a", "http://node-a", 100, 90, []MediaItem{itemA}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SyncMedia(ctx, "node-b", "http://node-b", 100, 80, []MediaItem{itemB}, now); err != nil {
		t.Fatal(err)
	}
	if videos, err := db.ListVideos(ctx, 0, "", 0, false, now); err != nil || len(videos) != 2 {
		t.Fatalf("online videos=%d err=%v", len(videos), err)
	}
	if _, err := db.pool.Exec(ctx, `UPDATE media_nodes SET last_seen_at=$2 WHERE name=$1`, "node-a", now.Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	videos, err := db.ListVideos(ctx, 0, "", 0, false, now)
	if err != nil || len(videos) != 1 || videos[0].NodeName != "node-b" {
		t.Fatalf("available videos=%+v err=%v", videos, err)
	}
	studios, err := db.ListStudios(ctx, now)
	if err != nil || len(studios) != 1 || studios[0].Name != "工作室乙" {
		t.Fatalf("studios=%+v err=%v", studios, err)
	}
}

func TestListNodesIncludesInventoryCounts(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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
	if _, err := db.pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	items := []MediaItem{
		{MediaKey: "ready-a", Studio: "Studio A", Title: "A", Compatibility: "ready"},
		{MediaKey: "ready-b", Studio: "Studio B", Title: "B", Compatibility: "ready"},
		{MediaKey: "unsupported", Studio: "Studio C", Title: "C", Compatibility: "unsupported"},
	}
	if _, err := db.SyncMedia(ctx, "nas", "http://nas", 100, 80, items, now); err != nil {
		t.Fatal(err)
	}
	nodes, err := db.ListNodes(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].StudioCount != 2 || nodes[0].VideoCount != 2 {
		t.Fatalf("nodes=%+v", nodes)
	}
	node, err := db.NodeByID(ctx, nodes[0].ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if node.StudioCount != 2 || node.VideoCount != 2 {
		t.Fatalf("node by id counts=%d/%d", node.StudioCount, node.VideoCount)
	}
}

func TestCleanupExpiredIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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
	if _, err := db.pool.Exec(ctx, `TRUNCATE audit_logs,playback_sessions,redeem_codes,payment_orders,video_entitlements,videos,studios,media_nodes,site_settings,wallet_entries,wallets,sessions,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	user, err := db.CreateUser(ctx, "cleanup@example.com", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	nodeID, err := db.SyncMedia(ctx, "node", "http://node", 100, 90, []MediaItem{{MediaKey: "video", Studio: "studio", Title: "video", VideoCodec: "h264", AudioCodec: "aac", Compatibility: "ready"}}, now)
	if err != nil || nodeID == 0 {
		t.Fatalf("node=%d err=%v", nodeID, err)
	}
	var videoID int64
	if err := db.pool.QueryRow(ctx, `SELECT id FROM videos LIMIT 1`).Scan(&videoID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,'expired',$3),($4,$2,'active',$5)`, []byte("expired"), user.ID, now.Add(-time.Minute), []byte("active"), now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO playback_sessions(id,user_id,video_id,expires_at,revoked_at,created_at) VALUES('expired',$1,$2,$3,NULL,$4),('active',$1,$2,$5,NULL,$4),('old-revoked',$1,$2,$5,$6,$4)`, user.ID, videoID, now.Add(-time.Minute), now.Add(-10*time.Hour), now.Add(time.Hour), now.Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.CleanupExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	var sessions, playbacks int
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM playback_sessions`).Scan(&playbacks); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || playbacks != 1 {
		t.Fatalf("remaining sessions=%d playbacks=%d", sessions, playbacks)
	}
}

func videoIDs(videos []Video) string {
	var result strings.Builder
	for _, video := range videos {
		fmt.Fprintf(&result, "%d,", video.ID)
	}
	return result.String()
}
