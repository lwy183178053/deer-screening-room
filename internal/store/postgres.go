package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Postgres struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (s *Postgres) Close() { s.pool.Close() }

func (s *Postgres) Migrate(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// Keep initialization serialized across gateway processes sharing a database.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('deerroom.schema_migrations'))`); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('deerroom.schema_migrations'))`)
	}()
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		version := entry.Name()
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Postgres) CreateUser(ctx context.Context, email, passwordHash string, admin bool) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var user User
	err = tx.QueryRow(ctx, `INSERT INTO users(email,password_hash,is_admin) VALUES(lower($1),$2,$3) RETURNING id,email,is_admin,enabled,created_at`, email, passwordHash, admin).
		Scan(&user.ID, &user.Email, &user.IsAdmin, &user.Enabled, &user.CreatedAt)
	if isUnique(err) {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO wallets(user_id) VALUES($1)`, user.ID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Postgres) UserByEmail(ctx context.Context, email string) (UserAuth, error) {
	var user UserAuth
	err := s.pool.QueryRow(ctx, `SELECT id,email,password_hash,is_admin,enabled,created_at FROM users WHERE lower(email)=lower($1)`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.IsAdmin, &user.Enabled, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserAuth{}, ErrNotFound
	}
	return user, err
}

func (s *Postgres) EnsureAdmin(ctx context.Context, email, passwordHash string) error {
	if email == "" || passwordHash == "" {
		return nil
	}
	result, err := s.pool.Exec(ctx, `UPDATE users SET is_admin=TRUE,updated_at=now() WHERE lower(email)=lower($1)`, email)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		return nil
	}
	_, err = s.CreateUser(ctx, email, passwordHash, true)
	if errors.Is(err, ErrConflict) {
		return nil
	}
	return err
}

func (s *Postgres) ListUsers(ctx context.Context, query string, pageNumber, pageSize int, now time.Time) (UserPage, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	args := []any{}
	where := "TRUE"
	var allTotal int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&allTotal); err != nil {
		return UserPage{}, err
	}
	if query = strings.TrimSpace(query); query != "" {
		args = append(args, "%"+query+"%")
		where = "u.email ILIKE $1"
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users u WHERE `+where, args...).Scan(&total); err != nil {
		return UserPage{}, err
	}
	statement := `SELECT u.id,u.email,u.is_admin,u.enabled,u.created_at,w.balance FROM users u JOIN wallets w ON w.user_id=u.id WHERE ` + where + ` ORDER BY u.created_at DESC,u.id DESC LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, pageSize, (pageNumber-1)*pageSize)
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil {
		return UserPage{}, err
	}
	defer rows.Close()
	result := []Account{}
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.Email, &account.IsAdmin, &account.Enabled, &account.CreatedAt, &account.Balance); err != nil {
			return UserPage{}, err
		}
		result = append(result, account)
	}
	if err := rows.Err(); err != nil {
		return UserPage{}, err
	}
	return UserPage{Users: result, Page: pageNumber, PageSize: pageSize, Total: total, AllTotal: allTotal}, nil
}

func (s *Postgres) SetUserEnabled(ctx context.Context, userID int64, enabled bool) error {
	result, err := s.pool.Exec(ctx, `UPDATE users SET enabled=$2,updated_at=now() WHERE id=$1`, userID, enabled)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if !enabled {
		_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	}
	return nil
}

func (s *Postgres) ResetUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, userID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) CreateSession(ctx context.Context, tokenHash []byte, userID int64, csrf string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)`, tokenHash, userID, csrf, expiresAt)
	return err
}

func (s *Postgres) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}

func (s *Postgres) AccountBySession(ctx context.Context, tokenHash []byte, now time.Time) (Account, string, error) {
	var account Account
	var csrf string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id,u.email,u.is_admin,u.enabled,u.created_at,w.balance,s.csrf_token
		FROM sessions s JOIN users u ON u.id=s.user_id JOIN wallets w ON w.user_id=u.id
		WHERE s.token_hash=$1 AND s.expires_at>$2`, tokenHash, now).
		Scan(&account.ID, &account.Email, &account.IsAdmin, &account.Enabled, &account.CreatedAt, &account.Balance, &csrf)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, "", ErrNotFound
	}
	if err != nil {
		return Account{}, "", err
	}
	return account, csrf, nil
}

func (s *Postgres) AccountByUserID(ctx context.Context, userID int64, now time.Time) (Account, error) {
	var account Account
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.email,u.is_admin,u.enabled,u.created_at,w.balance FROM users u JOIN wallets w ON w.user_id=u.id WHERE u.id=$1`, userID).
		Scan(&account.ID, &account.Email, &account.IsAdmin, &account.Enabled, &account.CreatedAt, &account.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return account, err
}

func (s *Postgres) ListWalletEntries(ctx context.Context, userID int64, limit int) ([]WalletEntry, error) {
	pageSize := limit
	if pageSize < 1 {
		pageSize = 20
	}
	page, err := s.ListWalletEntriesPage(ctx, userID, 1, pageSize)
	return page.Entries, err
}

func (s *Postgres) ListWalletEntriesPage(ctx context.Context, userID int64, pageNumber, pageSize int) (WalletEntryPage, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_entries WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return WalletEntryPage{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,delta,kind,reference_id,description,created_at FROM wallet_entries WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, userID, pageSize, (pageNumber-1)*pageSize)
	if err != nil {
		return WalletEntryPage{}, err
	}
	defer rows.Close()
	entries := []WalletEntry{}
	for rows.Next() {
		var entry WalletEntry
		if err := rows.Scan(&entry.ID, &entry.Delta, &entry.Kind, &entry.ReferenceID, &entry.Description, &entry.CreatedAt); err != nil {
			return WalletEntryPage{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return WalletEntryPage{}, err
	}
	return WalletEntryPage{Entries: entries, Page: pageNumber, PageSize: pageSize, Total: total}, nil
}

func (s *Postgres) AddAudit(ctx context.Context, actor int64, action, targetType, targetID string, detail any) error {
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,detail) VALUES($1,$2,$3,$4,$5)`, nullableActor(actor), action, targetType, targetID, body)
	return err
}

func nullableActor(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func isUnique(err error) bool { return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") }
