package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const redeemNoticeKey = "redeem_notice"
const userStreamBPSKey = "user_stream_bps"

func (s *Postgres) RedeemNotice(ctx context.Context) (string, error) {
	var content string
	err := s.pool.QueryRow(ctx, `SELECT value FROM site_settings WHERE key=$1`, redeemNoticeKey).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return content, err
}

func (s *Postgres) SetRedeemNotice(ctx context.Context, actorID int64, content string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO site_settings(key,value,updated_by,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`, redeemNoticeKey, content, actorID, now)
	return err
}

func (s *Postgres) UserStreamBPS(ctx context.Context) (int64, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `SELECT value FROM site_settings WHERE key=$1`, userStreamBPSKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	bps, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || bps < 1 {
		return 0, fmt.Errorf("invalid user stream bandwidth setting")
	}
	return bps, nil
}

func (s *Postgres) SetUserStreamBPS(ctx context.Context, actorID, bps int64, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO site_settings(key,value,updated_by,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`, userStreamBPSKey, strconv.FormatInt(bps, 10), actorID, now)
	return err
}
