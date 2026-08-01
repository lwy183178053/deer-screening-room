package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) Redeem(ctx context.Context, userID int64, codeHash []byte, now time.Time) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var codeID, credits int64
	var redeemedBy *int64
	err = tx.QueryRow(ctx, `SELECT id,credits,redeemed_by FROM redeem_codes WHERE code_hash=$1 FOR UPDATE`, codeHash).
		Scan(&codeID, &credits, &redeemedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrCodeInvalid
	}
	if err != nil {
		return 0, err
	}
	if redeemedBy != nil {
		return 0, ErrCodeUsed
	}
	if _, err := tx.Exec(ctx, `UPDATE redeem_codes SET redeemed_by=$1,redeemed_at=$2 WHERE id=$3`, userID, now, codeID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance=balance+$1,updated_at=$2 WHERE user_id=$3`, credits, now, userID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO wallet_entries(user_id,delta,kind,reference_id,description,created_at) VALUES($1,$2,'redeem',$3,'兑换码充值',$4)`, userID, credits, fmt.Sprint(codeID), now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return credits, nil
}

func (s *Postgres) AdjustCredits(ctx context.Context, actorID, userID, delta int64, reason, requestID string, now time.Time) (Account, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, false, err
	}
	defer tx.Rollback(ctx)

	var balance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance); errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, ErrNotFound
	} else if err != nil {
		return Account{}, false, err
	}
	description := strings.TrimSpace(reason)
	if description == "" {
		if delta > 0 {
			description = "管理员增加鹿币"
		} else {
			description = "管理员扣减鹿币"
		}
	}

	var existingUserID, existingDelta int64
	var existingDescription string
	err = tx.QueryRow(ctx, `SELECT user_id,delta,description FROM wallet_entries WHERE kind='admin_adjustment' AND reference_id=$1`, requestID).Scan(&existingUserID, &existingDelta, &existingDescription)
	if err == nil {
		if existingUserID != userID || existingDelta != delta || existingDescription != description {
			return Account{}, false, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Account{}, false, err
		}
		account, err := s.AccountByUserID(ctx, userID, now)
		return account, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, err
	}
	if delta == 0 || balance+delta < 0 {
		return Account{}, false, ErrInsufficient
	}
	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance=balance+$1,updated_at=$2 WHERE user_id=$3`, delta, now, userID); err != nil {
		return Account{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO wallet_entries(user_id,delta,kind,reference_id,description,created_at) VALUES($1,$2,'admin_adjustment',$3,$4,$5)`, userID, delta, requestID, description, now); err != nil {
		return Account{}, false, err
	}
	detail, err := json.Marshal(map[string]any{"delta": delta, "reason": strings.TrimSpace(reason), "request_id": requestID})
	if err != nil {
		return Account{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,detail) VALUES($1,'wallet.admin_adjusted','user',$2,$3)`, actorID, fmt.Sprint(userID), detail); err != nil {
		return Account{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, false, err
	}
	account, err := s.AccountByUserID(ctx, userID, now)
	return account, true, err
}

func (s *Postgres) UnlockVideo(ctx context.Context, userID, videoID int64, reference string, now time.Time) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var published, available bool
	if err := tx.QueryRow(ctx, `SELECT published,available FROM videos WHERE id=$1`, videoID).Scan(&published, &available); errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	} else if err != nil {
		return false, err
	}
	if !published || !available {
		return false, ErrUnavailable
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_entitlements WHERE user_id=$1 AND video_id=$2)`, userID, videoID).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, tx.Commit(ctx)
	}
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return false, err
	}
	if balance < 1 {
		return false, ErrInsufficient
	}
	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance=balance-1,updated_at=$2 WHERE user_id=$1`, userID, now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO video_entitlements(user_id,video_id,created_at) VALUES($1,$2,$3)`, userID, videoID, now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO wallet_entries(user_id,delta,kind,reference_id,description,created_at) VALUES($1,-1,'video_unlock',$2,'永久解锁视频',$3)`, userID, reference, now); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s *Postgres) CreateRedeemCodes(ctx context.Context, actor, credits int64, hashes [][]byte, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	batch := &pgx.Batch{}
	for _, hash := range hashes {
		batch.Queue(`INSERT INTO redeem_codes(code_hash,credits,created_by,created_at) VALUES($1,$2,$3,$4)`, hash, credits, actor, now)
	}
	results := tx.SendBatch(ctx, batch)
	for range hashes {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return err
		}
	}
	if err := results.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) ListRedeemCodes(ctx context.Context, status, query string, codeHash []byte, pageNumber, pageSize int) (RedeemCodePage, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	var counts RedeemCodeCounts
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE redeemed_by IS NOT NULL),COUNT(*) FILTER (WHERE redeemed_by IS NULL) FROM redeem_codes`).Scan(&counts.All, &counts.Used, &counts.Unused); err != nil {
		return RedeemCodePage{}, err
	}
	args := []any{}
	where := []string{"TRUE"}
	if status == "used" {
		where = append(where, "c.redeemed_by IS NOT NULL")
	} else if status == "unused" {
		where = append(where, "c.redeemed_by IS NULL")
	}
	if len(codeHash) > 0 {
		args = append(args, codeHash)
		where = append(where, fmt.Sprintf("c.code_hash=$%d", len(args)))
	} else if query != "" {
		args = append(args, "%"+query+"%")
		where = append(where, fmt.Sprintf("u.email ILIKE $%d", len(args)))
	}
	from := ` FROM redeem_codes c LEFT JOIN users u ON u.id=c.redeemed_by WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return RedeemCodePage{}, err
	}
	args = append(args, pageSize, (pageNumber-1)*pageSize)
	rows, err := s.pool.Query(ctx, `SELECT c.id,c.credits,c.redeemed_by IS NOT NULL,u.email,c.redeemed_at,c.created_at`+from+fmt.Sprintf(` ORDER BY c.created_at DESC,c.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return RedeemCodePage{}, err
	}
	defer rows.Close()
	result := []RedeemCode{}
	for rows.Next() {
		var item RedeemCode
		if err := rows.Scan(&item.ID, &item.Credits, &item.Used, &item.RedeemedByEmail, &item.RedeemedAt, &item.CreatedAt); err != nil {
			return RedeemCodePage{}, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return RedeemCodePage{}, err
	}
	return RedeemCodePage{Codes: result, Page: pageNumber, PageSize: pageSize, Total: total, Counts: counts}, nil
}
