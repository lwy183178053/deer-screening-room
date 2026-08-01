package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const redeemNoticeKey = "redeem_notice"
const p2pEnabledKey = "p2p_enabled"

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

func (s *Postgres) P2PEnabled(ctx context.Context) (bool, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `SELECT value FROM site_settings WHERE key=$1`, p2pEnabledKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return raw == "true", nil
}

func (s *Postgres) SetP2PEnabled(ctx context.Context, actorID int64, enabled bool, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO site_settings(key,value,updated_by,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`, p2pEnabledKey, strconv.FormatBool(enabled), actorID, now)
	return err
}
