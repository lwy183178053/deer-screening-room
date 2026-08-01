package store

import (
	"context"
	"time"
)

func (s *Postgres) CleanupExpired(ctx context.Context, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at<$1`, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM playback_sessions WHERE expires_at<$1 OR (revoked_at IS NOT NULL AND revoked_at<$1::timestamptz-interval '7 days')`, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
