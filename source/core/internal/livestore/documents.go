package livestore

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"docssearch/core/internal/chunk"
)

type UploadInput struct {
	FileName    string
	ContentType string
	Body        []byte
	Title       string
	Edition     string
	AccessLevel int
}

type UploadResult struct {
	DocumentID string
	Edition    string
}

func (store *LiveStore) Index_UploadedDocument(ctx context.Context, token string, input UploadInput) (UploadResult, error) {
	if _, err := store.Find_AccountByToken(ctx, token); err != nil {
		return UploadResult{}, err
	}
	plain, err := chunk.Extract_DocumentText(input.FileName, input.Body)
	if errors.Is(err, chunk.ErrUnsupportedFile) {
		return UploadResult{}, ErrUnsupportedFile
	}
	if err != nil {
		return UploadResult{}, ErrUnsupportedFile
	}
	pieces := chunk.Split_DocumentText(plain)
	contents := make([]string, 0, len(pieces))
	for _, piece := range pieces {
		contents = append(contents, piece.Content)
	}
	vectors, err := store.embed_Texts(contents)
	if err != nil {
		return UploadResult{}, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return UploadResult{}, err
	}
	defer tx.Rollback(ctx)

	var documentID string
	err = tx.QueryRow(ctx, `SELECT document_id FROM documents WHERE title = $1`, input.Title).Scan(&documentID)
	if errors.Is(err, pgx.ErrNoRows) {
		number, nextError := next_IDTx(ctx, tx, counterDocument)
		if nextError != nil {
			return UploadResult{}, nextError
		}
		documentID = "doc-" + strconv.Itoa(number)
		_, err = tx.Exec(ctx, `
			INSERT INTO documents (document_id, title, edition, access_level, file_name, content_type, body)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			documentID, input.Title, input.Edition, input.AccessLevel, input.FileName, input.ContentType, input.Body)
	} else if err != nil {
		return UploadResult{}, err
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE documents
			SET edition = $2, access_level = $3, file_name = $4, content_type = $5, body = $6
			WHERE document_id = $1`,
			documentID, input.Edition, input.AccessLevel, input.FileName, input.ContentType, input.Body)
	}
	if err != nil {
		if is_UniqueViolation(err) {
			return UploadResult{}, err
		}
		return UploadResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE document_id = $1`, documentID); err != nil {
		return UploadResult{}, err
	}
	for index, piece := range pieces {
		_, err = tx.Exec(ctx, `
			INSERT INTO chunks (document_id, section, paragraph, appendix, access_level, content, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7::vector)`,
			documentID, piece.Section, piece.Paragraph, piece.Appendix, input.AccessLevel, piece.Content, format_Vector(vectors[index]))
		if err != nil {
			return UploadResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{DocumentID: documentID, Edition: input.Edition}, nil
}

func (store *LiveStore) embed_Texts(contents []string) ([][]float32, error) {
	if len(contents) == 0 {
		return nil, nil
	}
	if store.embedder == nil {
		return nil, ErrEmbedUnavailable
	}
	vectors, err := store.embedder.Embed_Texts(contents)
	if err != nil || len(vectors) != len(contents) {
		return nil, ErrEmbedUnavailable
	}
	for _, vector := range vectors {
		if len(vector) != store.dimensions {
			return nil, ErrEmbedSize
		}
	}
	return vectors, nil
}

func (store *LiveStore) List_VisibleDocuments(ctx context.Context, token string) ([]DocumentLine, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT document_id, title, edition, access_level
		FROM documents
		WHERE access_level <= $1
		ORDER BY document_id`, account.AccessLevel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := []DocumentLine{}
	for rows.Next() {
		var line DocumentLine
		if err := rows.Scan(&line.DocumentID, &line.Title, &line.Edition, &line.AccessLevel); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (store *LiveStore) Read_DocumentFile(ctx context.Context, token, documentID string) (DocumentFile, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return DocumentFile{}, err
	}
	var file DocumentFile
	var accessLevel int
	err = store.pool.QueryRow(ctx, `
		SELECT access_level, file_name, content_type, body
		FROM documents WHERE document_id = $1`, documentID).Scan(&accessLevel, &file.FileName, &file.ContentType, &file.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return DocumentFile{}, ErrNotFound
	}
	if err != nil {
		return DocumentFile{}, err
	}
	if account.AccessLevel < accessLevel {
		return DocumentFile{}, ErrAccessDenied
	}
	return file, nil
}

func (store *LiveStore) Find_ChunksBySimilarity(ctx context.Context, embedding []float32, accessLevel, limit int) ([]SimilarChunk, error) {
	if len(embedding) != store.dimensions {
		return nil, ErrEmbedSize
	}
	if limit <= 0 {
		limit = 5
	}
	rows, err := store.pool.Query(ctx, `
		SELECT d.document_id, d.title, d.edition, c.section, c.paragraph, c.appendix, c.content,
			c.embedding <=> $1::vector AS distance
		FROM chunks c
		JOIN documents d ON d.document_id = c.document_id
		WHERE c.access_level <= $2
		ORDER BY c.embedding <=> $1::vector
		LIMIT $3`, format_Vector(embedding), accessLevel, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := []SimilarChunk{}
	for rows.Next() {
		var chunkRow SimilarChunk
		if err := rows.Scan(&chunkRow.DocumentID, &chunkRow.Title, &chunkRow.Edition, &chunkRow.Section, &chunkRow.Paragraph, &chunkRow.Appendix, &chunkRow.Content, &chunkRow.Distance); err != nil {
			return nil, err
		}
		found = append(found, chunkRow)
	}
	return found, rows.Err()
}

func format_Vector(values []float32) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for index, value := range values {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func next_IDTx(ctx context.Context, tx queryRower, name string) (int, error) {
	var value int
	err := tx.QueryRow(ctx, `UPDATE id_counters SET value = value + 1 WHERE name = $1 RETURNING value - 1`, name).Scan(&value)
	return value, err
}
