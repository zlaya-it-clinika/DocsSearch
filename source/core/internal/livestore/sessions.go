package livestore

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type AuditLine struct {
	EntryID     string
	UserID      string
	DisplayName string
	Question    string
	CreatedAt   time.Time
	Documents   []byte
}

func (store *LiveStore) Create_Session(ctx context.Context, token string) (string, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return "", err
	}
	number, err := store.next_ID(ctx, counterSession)
	if err != nil {
		return "", err
	}
	sessionID := "session-" + strconv.Itoa(number)
	_, err = store.pool.Exec(ctx, `
		INSERT INTO sessions (session_id, owner_user_id, template_step)
		VALUES ($1, $2, 0)`, sessionID, account.UserID)
	return sessionID, err
}

func (store *LiveStore) List_SessionMessages(ctx context.Context, token, sessionID string) ([][]byte, error) {
	if _, _, err := store.Load_SessionForMessage(ctx, token, sessionID); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT body FROM session_messages WHERE session_id = $1 ORDER BY position`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := [][]byte{}
	for rows.Next() {
		var body []byte
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		messages = append(messages, body)
	}
	return messages, rows.Err()
}

func (store *LiveStore) Load_SessionForMessage(ctx context.Context, token, sessionID string) (Account, int, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return Account{}, 0, err
	}
	var ownerID string
	var templateStep int
	err = store.pool.QueryRow(ctx, `
		SELECT owner_user_id, template_step FROM sessions WHERE session_id = $1`, sessionID).Scan(&ownerID, &templateStep)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, 0, ErrNotFound
	}
	if err != nil {
		return Account{}, 0, err
	}
	if ownerID != account.UserID {
		return Account{}, 0, ErrAccessDenied
	}
	return account, templateStep, nil
}

func (store *LiveStore) Save_SessionExchange(ctx context.Context, sessionID string, templateStep int, userBody, assistantBody, auditDocuments []byte, userID, displayName, question string) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE sessions SET template_step = $2 WHERE session_id = $1`, sessionID, templateStep); err != nil {
		return err
	}
	var position int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(position), 0) FROM session_messages WHERE session_id = $1`, sessionID).Scan(&position); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO session_messages (session_id, position, body) VALUES ($1, $2, $3::jsonb)`, sessionID, position+1, string(userBody)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO session_messages (session_id, position, body) VALUES ($1, $2, $3::jsonb)`, sessionID, position+2, string(assistantBody)); err != nil {
		return err
	}
	number, err := next_IDTx(ctx, tx, counterAudit)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_entries (entry_id, user_id, display_name, question, created_at, documents)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		"audit-"+strconv.Itoa(number), userID, displayName, question, time.Now().UTC(), string(auditDocuments))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *LiveStore) List_Audit(ctx context.Context, token string) ([]AuditLine, error) {
	if _, err := store.require_Admin(ctx, token); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT entry_id, user_id, display_name, question, created_at, documents
		FROM audit_entries
		ORDER BY created_at ASC, entry_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []AuditLine{}
	for rows.Next() {
		var entry AuditLine
		if err := rows.Scan(&entry.EntryID, &entry.UserID, &entry.DisplayName, &entry.Question, &entry.CreatedAt, &entry.Documents); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
