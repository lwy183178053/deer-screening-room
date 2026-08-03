package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) SyncMedia(ctx context.Context, nodeName, baseURL string, totalBytes, availableBytes int64, items []MediaItem, now time.Time) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var nodeID int64
	err = tx.QueryRow(ctx, `INSERT INTO media_nodes(name,base_url,online,total_bytes,available_bytes,last_seen_at,updated_at) VALUES($1,$2,true,$3,$4,$5,$5) ON CONFLICT(name) DO UPDATE SET base_url=EXCLUDED.base_url,online=true,total_bytes=EXCLUDED.total_bytes,available_bytes=EXCLUDED.available_bytes,last_seen_at=EXCLUDED.last_seen_at,updated_at=EXCLUDED.updated_at RETURNING id`, nodeName, baseURL, max(totalBytes, 0), max(availableBytes, 0), now).Scan(&nodeID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE videos SET available=false,updated_at=$2 WHERE node_id=$1`, nodeID, now); err != nil {
		return 0, err
	}
	studioIDs := make(map[string]int64)
	for _, item := range items {
		if _, ok := studioIDs[item.Studio]; ok {
			continue
		}
		var studioID int64
		if err := tx.QueryRow(ctx, `INSERT INTO studios(node_id,source_name,name) VALUES($1,$2,$2) ON CONFLICT(node_id,source_name) DO UPDATE SET updated_at=now() RETURNING id`, nodeID, item.Studio).Scan(&studioID); err != nil {
			return 0, err
		}
		studioIDs[item.Studio] = studioID
	}
	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(`
			INSERT INTO videos(node_id,studio_id,media_key,source_title,title,poster_key,duration_ms,size_bytes,bit_rate,width,height,video_codec,audio_codec,compatibility,published,available,last_seen_at)
			VALUES($1,$2,$3,$4,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13='ready',true,$14)
			ON CONFLICT(node_id,media_key) DO UPDATE SET studio_id=EXCLUDED.studio_id,source_title=EXCLUDED.source_title,title=EXCLUDED.title,poster_key=EXCLUDED.poster_key,duration_ms=EXCLUDED.duration_ms,size_bytes=EXCLUDED.size_bytes,bit_rate=EXCLUDED.bit_rate,width=EXCLUDED.width,height=EXCLUDED.height,video_codec=EXCLUDED.video_codec,audio_codec=EXCLUDED.audio_codec,compatibility=EXCLUDED.compatibility,published=EXCLUDED.compatibility='ready',available=true,last_seen_at=EXCLUDED.last_seen_at,updated_at=EXCLUDED.last_seen_at`,
			nodeID, studioIDs[item.Studio], item.MediaKey, item.Title, item.PosterKey, item.DurationMS, item.SizeBytes, item.BitRate, item.Width, item.Height, item.VideoCodec, item.AudioCodec, item.Compatibility, now)
	}
	if batch.Len() > 0 {
		results := tx.SendBatch(ctx, batch)
		for range items {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return 0, err
			}
		}
		if err := results.Close(); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return nodeID, nil
}

func (s *Postgres) HeartbeatNode(ctx context.Context, name, baseURL string, totalBytes, availableBytes int64, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO media_nodes(name,base_url,online,total_bytes,available_bytes,last_seen_at,updated_at) VALUES($1,$2,true,$3,$4,$5,$5) ON CONFLICT(name) DO UPDATE SET base_url=EXCLUDED.base_url,online=true,total_bytes=EXCLUDED.total_bytes,available_bytes=EXCLUDED.available_bytes,last_seen_at=EXCLUDED.last_seen_at,updated_at=EXCLUDED.updated_at`, name, baseURL, max(totalBytes, 0), max(availableBytes, 0), now)
	return err
}

func (s *Postgres) ListStudios(ctx context.Context, now time.Time) ([]Studio, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.id,s.name,COUNT(v.id) FROM studios s JOIN videos v ON v.studio_id=s.id JOIN media_nodes n ON n.id=v.node_id WHERE s.published AND v.published AND v.available AND v.compatibility='ready' AND n.online AND n.last_seen_at>$1::timestamptz-interval '90 seconds' GROUP BY s.id ORDER BY s.name`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Studio{}
	for rows.Next() {
		var item Studio
		if err := rows.Scan(&item.ID, &item.Name, &item.VideoCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Postgres) ListVideos(ctx context.Context, userID int64, query string, studioID int64, purchasedOnly bool, now time.Time) ([]Video, error) {
	page, err := s.ListVideosPage(ctx, userID, query, studioID, purchasedOnly, now, 1, 0, 0)
	return page.Videos, err
}

func (s *Postgres) ListVideosPage(ctx context.Context, userID int64, query string, studioID int64, purchasedOnly bool, now time.Time, pageNumber, pageSize int, seed int64) (VideoPage, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	if pageSize < 0 {
		pageSize = 0
	}
	args := []any{userID, now}
	where := []string{"v.published", "v.available", "v.compatibility='ready'", "n.online", "n.last_seen_at>$2::timestamptz-interval '90 seconds'"}
	if query != "" {
		args = append(args, "%"+query+"%")
		placeholder := "$" + itoa(len(args))
		where = append(where, "(v.title ILIKE "+placeholder+" OR s.name ILIKE "+placeholder+")")
	}
	if studioID > 0 {
		args = append(args, studioID)
		where = append(where, "v.studio_id=$"+itoa(len(args)))
	}
	if purchasedOnly {
		where = append(where, "e.user_id IS NOT NULL")
	}
	from := ` FROM videos v JOIN studios s ON s.id=v.studio_id JOIN media_nodes n ON n.id=v.node_id LEFT JOIN video_entitlements e ON e.video_id=v.id AND e.user_id=$1 WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return VideoPage{}, err
	}
	listArgs := append([]any{}, args...)
	nowPlaceholder := "$2"
	orderBy := `v.updated_at DESC,v.id DESC`
	if seed > 0 {
		listArgs = append(listArgs, seed)
		orderBy = `hashtextextended(v.id::text,$` + itoa(len(listArgs)) + `),v.id`
	}
	statement := `SELECT v.id,v.node_id,n.name,n.base_url,v.studio_id,s.name,v.media_key,v.source_title,v.title,v.poster_key,v.duration_ms,v.size_bytes,v.bit_rate,v.width,v.height,v.video_codec,v.audio_codec,v.compatibility,v.published,v.available AND n.online AND COALESCE(n.last_seen_at>(` + nowPlaceholder + `::timestamptz - interval '90 seconds'),false),v.updated_at,e.user_id IS NOT NULL,e.user_id IS NOT NULL` + from + ` ORDER BY ` + orderBy
	if pageSize > 0 {
		statement += ` LIMIT $` + itoa(len(listArgs)+1) + ` OFFSET $` + itoa(len(listArgs)+2)
		listArgs = append(listArgs, pageSize, (pageNumber-1)*pageSize)
	}
	rows, err := s.pool.Query(ctx, statement, listArgs...)
	if err != nil {
		return VideoPage{}, err
	}
	defer rows.Close()
	result := []Video{}
	for rows.Next() {
		item, err := scanVideo(rows)
		if err != nil {
			return VideoPage{}, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return VideoPage{}, err
	}
	return VideoPage{Videos: result, Page: pageNumber, PageSize: pageSize, Total: total}, nil
}

func (s *Postgres) VideoByID(ctx context.Context, userID, videoID int64, now time.Time) (Video, error) {
	statement := `SELECT v.id,v.node_id,n.name,n.base_url,v.studio_id,s.name,v.media_key,v.source_title,v.title,v.poster_key,v.duration_ms,v.size_bytes,v.bit_rate,v.width,v.height,v.video_codec,v.audio_codec,v.compatibility,v.published,v.available AND n.online AND COALESCE(n.last_seen_at>($3::timestamptz - interval '90 seconds'),false),v.updated_at,e.user_id IS NOT NULL,e.user_id IS NOT NULL FROM videos v JOIN studios s ON s.id=v.studio_id JOIN media_nodes n ON n.id=v.node_id LEFT JOIN video_entitlements e ON e.video_id=v.id AND e.user_id=$1 WHERE v.id=$2 AND v.published AND v.compatibility='ready'`
	item, err := scanVideo(s.pool.QueryRow(ctx, statement, userID, videoID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	return item, err
}

func (s *Postgres) CreatePlayback(ctx context.Context, id string, userID, videoID int64, expires, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedUserID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var canPlay, available bool
	err = tx.QueryRow(ctx, `SELECT e.user_id IS NOT NULL,v.published AND v.available AND v.compatibility='ready' AND n.online AND COALESCE(n.last_seen_at>($3::timestamptz - interval '90 seconds'),false) FROM videos v JOIN media_nodes n ON n.id=v.node_id LEFT JOIN video_entitlements e ON e.video_id=v.id AND e.user_id=$1 WHERE v.id=$2`, userID, videoID, now).Scan(&canPlay, &available)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !available {
		return ErrUnavailable
	}
	if !canPlay {
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx, `UPDATE playback_sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO playback_sessions(id,user_id,video_id,expires_at,created_at) VALUES($1,$2,$3,$4,$5)`, id, userID, videoID, expires, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) PlaybackVideo(ctx context.Context, playbackID string, userID int64, now time.Time) (Video, error) {
	var videoID int64
	err := s.pool.QueryRow(ctx, `SELECT video_id FROM playback_sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>$3`, playbackID, userID, now).Scan(&videoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	if err != nil {
		return Video{}, err
	}
	return s.VideoByID(ctx, userID, videoID, now)
}

func (s *Postgres) ListNodes(ctx context.Context, now time.Time) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id,n.name,n.base_url,COALESCE(p.wireguard_address::text,''),p.node_id IS NOT NULL,
			COALESCE(p.revoked_at IS NOT NULL,false),COALESCE(p.bundle_downloaded_at IS NOT NULL,false),
			n.online AND COALESCE(n.last_seen_at>$1,false),n.total_bytes,n.available_bytes,
			COALESCE(c.studio_count,0),COALESCE(c.video_count,0),n.last_seen_at
		FROM media_nodes n
		LEFT JOIN media_node_provisioning p ON p.node_id=n.id
		LEFT JOIN (
			SELECT v.node_id,
				COUNT(DISTINCT s.id) FILTER (WHERE s.published AND v.published AND v.available) AS studio_count,
				COUNT(v.id) FILTER (WHERE v.published AND v.available) AS video_count
			FROM videos v
			JOIN studios s ON s.id=v.studio_id
			GROUP BY v.node_id
		) c ON c.node_id=n.id
		ORDER BY n.name`, now.Add(-90*time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Node{}
	for rows.Next() {
		var item Node
		if err := rows.Scan(&item.ID, &item.Name, &item.BaseURL, &item.WireGuardAddress, &item.Provisioned, &item.Revoked, &item.BundleDownloaded, &item.Online, &item.TotalBytes, &item.AvailableBytes, &item.StudioCount, &item.VideoCount, &item.LastSeenAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Postgres) NodeByID(ctx context.Context, id int64, now time.Time) (Node, error) {
	var item Node
	err := s.pool.QueryRow(ctx, `
		SELECT n.id,n.name,n.base_url,COALESCE(p.wireguard_address::text,''),p.node_id IS NOT NULL,
			COALESCE(p.revoked_at IS NOT NULL,false),COALESCE(p.bundle_downloaded_at IS NOT NULL,false),
			n.online AND COALESCE(n.last_seen_at>$2,false),n.total_bytes,n.available_bytes,
			COALESCE(c.studio_count,0),COALESCE(c.video_count,0),n.last_seen_at
		FROM media_nodes n
		LEFT JOIN media_node_provisioning p ON p.node_id=n.id
		LEFT JOIN (
			SELECT v.node_id,
				COUNT(DISTINCT s.id) FILTER (WHERE s.published AND v.published AND v.available) AS studio_count,
				COUNT(v.id) FILTER (WHERE v.published AND v.available) AS video_count
			FROM videos v
			JOIN studios s ON s.id=v.studio_id
			GROUP BY v.node_id
		) c ON c.node_id=n.id
		WHERE n.id=$1`, id, now.Add(-90*time.Second)).Scan(&item.ID, &item.Name, &item.BaseURL, &item.WireGuardAddress, &item.Provisioned, &item.Revoked, &item.BundleDownloaded, &item.Online, &item.TotalBytes, &item.AvailableBytes, &item.StudioCount, &item.VideoCount, &item.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	return item, err
}

func (s *Postgres) ListNodeBaseURLs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT base_url FROM media_nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var baseURL string
		if err := rows.Scan(&baseURL); err != nil {
			return nil, err
		}
		result = append(result, baseURL)
	}
	return result, rows.Err()
}

func (s *Postgres) CreateProvisionedNode(ctx context.Context, name, baseURL, address, publicKey, privateKeySealed, apiSealed, relaySealed string, now time.Time) (ProvisionedNode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProvisionedNode{}, err
	}
	defer tx.Rollback(ctx)
	var node ProvisionedNode
	err = tx.QueryRow(ctx, `INSERT INTO media_nodes(name,base_url,online,last_seen_at,updated_at) VALUES($1,$2,false,$3,$3) RETURNING id,name,base_url,online,total_bytes,available_bytes,last_seen_at`, name, baseURL, now).
		Scan(&node.ID, &node.Name, &node.BaseURL, &node.Online, &node.TotalBytes, &node.AvailableBytes, &node.LastSeenAt)
	if isUnique(err) {
		return ProvisionedNode{}, ErrConflict
	}
	if err != nil {
		return ProvisionedNode{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO media_node_provisioning(node_id,wireguard_address,wireguard_public_key,wireguard_private_key_sealed,api_token_sealed,relay_token_sealed) VALUES($1,$2,$3,$4,$5,$6)`, node.ID, address, publicKey, privateKeySealed, apiSealed, relaySealed)
	if isUnique(err) {
		return ProvisionedNode{}, ErrConflict
	}
	if err != nil {
		return ProvisionedNode{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProvisionedNode{}, err
	}
	node.WireGuardAddress, node.WireGuardPublicKey = address, publicKey
	node.Provisioned = true
	node.APITokenSealed, node.RelayTokenSealed = apiSealed, relaySealed
	return node, nil
}

func (s *Postgres) ProvisionedNodeByName(ctx context.Context, name string) (ProvisionedNode, error) {
	return s.provisionedNode(ctx, `WHERE n.name=$1`, name)
}

func (s *Postgres) ProvisionedNodeByID(ctx context.Context, id int64) (ProvisionedNode, error) {
	return s.provisionedNode(ctx, `WHERE n.id=$1`, id)
}

func (s *Postgres) provisionedNode(ctx context.Context, clause string, arg any) (ProvisionedNode, error) {
	var node ProvisionedNode
	err := s.pool.QueryRow(ctx, `SELECT n.id,n.name,n.base_url,n.online,n.total_bytes,n.available_bytes,n.last_seen_at,p.wireguard_address::text,p.wireguard_public_key,p.wireguard_private_key_sealed,p.api_token_sealed,p.relay_token_sealed,p.bundle_downloaded_at IS NOT NULL,p.revoked_at IS NOT NULL FROM media_nodes n JOIN media_node_provisioning p ON p.node_id=n.id `+clause, arg).
		Scan(&node.ID, &node.Name, &node.BaseURL, &node.Online, &node.TotalBytes, &node.AvailableBytes, &node.LastSeenAt, &node.WireGuardAddress, &node.WireGuardPublicKey, &node.WireGuardPrivateKeySealed, &node.APITokenSealed, &node.RelayTokenSealed, &node.BundleDownloaded, &node.Revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisionedNode{}, ErrNotFound
	}
	if err != nil {
		return ProvisionedNode{}, err
	}
	node.Provisioned = true
	return node, nil
}

func (s *Postgres) MarkNodeBundleDownloaded(ctx context.Context, id int64, now time.Time) error {
	result, err := s.pool.Exec(ctx, `UPDATE media_node_provisioning SET bundle_downloaded_at=$2 WHERE node_id=$1 AND bundle_downloaded_at IS NULL AND revoked_at IS NULL`, id, now)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Postgres) RotateProvisionedNode(ctx context.Context, id int64, apiSealed, relaySealed string) error {
	result, err := s.pool.Exec(ctx, `UPDATE media_node_provisioning SET api_token_sealed=$2,relay_token_sealed=$3,bundle_downloaded_at=NULL,revoked_at=NULL WHERE node_id=$1`, id, apiSealed, relaySealed)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Postgres) DeleteNode(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var nodeID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM media_nodes WHERE id=$1 FOR UPDATE`, id).Scan(&nodeID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	statements := []string{
		`DELETE FROM video_entitlements WHERE video_id IN (SELECT id FROM videos WHERE node_id=$1)`,
		`DELETE FROM playback_sessions WHERE video_id IN (SELECT id FROM videos WHERE node_id=$1)`,
		`DELETE FROM videos WHERE node_id=$1`,
		`DELETE FROM studios WHERE node_id=$1`,
		`DELETE FROM media_node_provisioning WHERE node_id=$1`,
		`DELETE FROM media_nodes WHERE id=$1`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, nodeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type rowScanner interface{ Scan(...any) error }

func scanVideo(row rowScanner) (Video, error) {
	var v Video
	err := row.Scan(&v.ID, &v.NodeID, &v.NodeName, &v.NodeURL, &v.StudioID, &v.StudioName, &v.MediaKey, &v.SourceTitle, &v.Title, &v.PosterKey, &v.DurationMS, &v.SizeBytes, &v.BitRate, &v.Width, &v.Height, &v.VideoCodec, &v.AudioCodec, &v.Compatibility, &v.Published, &v.Available, &v.UpdatedAt, &v.Unlocked, &v.CanPlay)
	return v, err
}
func itoa(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return ""
}
