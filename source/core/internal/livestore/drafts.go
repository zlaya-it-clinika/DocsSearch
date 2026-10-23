package livestore

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"docssearch/core/internal/texts"
)

const counterDraft = "draft"

func (store *LiveStore) Save_DraftFile(ctx context.Context, ownerID, fileName string, body []byte) (string, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	number, err := next_IDTx(ctx, tx, counterDraft)
	if err != nil {
		return "", err
	}
	draftID := "draft-" + strconv.Itoa(number)
	_, err = tx.Exec(ctx, `
		INSERT INTO draft_files (draft_id, owner_user_id, file_name, content_type, body)
		VALUES ($1, $2, $3, $4, $5)`,
		draftID, ownerID, fileName, texts.DocxContentType, body)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return draftID, nil
}

func (store *LiveStore) Read_DraftFile(ctx context.Context, userID, draftID string) (DocumentFile, error) {
	var ownerID string
	var file DocumentFile
	err := store.pool.QueryRow(ctx, `
		SELECT owner_user_id, file_name, content_type, body
		FROM draft_files WHERE draft_id = $1`, draftID).Scan(&ownerID, &file.FileName, &file.ContentType, &file.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return DocumentFile{}, ErrNotFound
	}
	if err != nil {
		return DocumentFile{}, err
	}
	if ownerID != userID {
		return DocumentFile{}, ErrAccessDenied
	}
	return file, nil
}
