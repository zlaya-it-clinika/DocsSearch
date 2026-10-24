package livestore

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"docssearch/core/internal/auth"
	"docssearch/core/internal/chunk"
	"docssearch/core/internal/contacts"
	"docssearch/core/internal/texts"
)

type UploadInput struct {
	FileName     string
	ContentType  string
	Body         []byte
	Title        string
	Edition      string
	AccessLevel  int
	Scope        string
	OwnerID      string
	ProcessID    string
	VisibleToAll bool
	RoleIDs      []string
}

type UploadResult struct {
	DocumentID string
	Edition    string
	Scope      string
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
	err = tx.QueryRow(ctx, `
		SELECT document_id FROM documents
		WHERE title = $1 AND scope = $2 AND (($2 = 'global') OR owner_id = $3)`,
		input.Title, input.Scope, input.OwnerID).Scan(&documentID)
	if errors.Is(err, pgx.ErrNoRows) {
		number, nextError := next_IDTx(ctx, tx, counterDocument)
		if nextError != nil {
			return UploadResult{}, nextError
		}
		documentID = "doc-" + strconv.Itoa(number)
		_, err = tx.Exec(ctx, `
			INSERT INTO documents (document_id, title, edition, access_level, scope, owner_id, process_id, uploaded_at, file_name, content_type, body, visible_to_all)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now(), $8, $9, $10, $11)`,
			documentID, input.Title, input.Edition, input.AccessLevel, input.Scope, input.OwnerID, upload_ProcessID(input.ProcessID), input.FileName, input.ContentType, input.Body, input.VisibleToAll)
	} else if err != nil {
		return UploadResult{}, err
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE documents
			SET edition = $2, access_level = $3, process_id = $4, file_name = $5, content_type = $6, body = $7, uploaded_at = now(), visible_to_all = $8
			WHERE document_id = $1`,
			documentID, input.Edition, input.AccessLevel, upload_ProcessID(input.ProcessID), input.FileName, input.ContentType, input.Body, input.VisibleToAll)
	}
	if err != nil {
		if is_UniqueViolation(err) {
			return UploadResult{}, err
		}
		return UploadResult{}, err
	}
	if err := check_RoleIDs(ctx, tx, input.RoleIDs); err != nil {
		return UploadResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM document_roles WHERE document_id = $1`, documentID); err != nil {
		return UploadResult{}, err
	}
	for _, roleID := range input.RoleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO document_roles (document_id, role_id) VALUES ($1, $2)`, documentID, roleID); err != nil {
			return UploadResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE document_id = $1`, documentID); err != nil {
		return UploadResult{}, err
	}
	for index, piece := range pieces {
		_, err = tx.Exec(ctx, `
			INSERT INTO chunks (document_id, section, paragraph, appendix, access_level, scope, owner_id, content, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::vector)`,
			documentID, piece.Section, piece.Paragraph, piece.Appendix, input.AccessLevel, input.Scope, input.OwnerID, piece.Content, format_Vector(vectors[index]))
		if err != nil {
			return UploadResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{DocumentID: documentID, Edition: input.Edition, Scope: input.Scope}, nil
}

func upload_ProcessID(processID string) string {
	if strings.TrimSpace(processID) == "" {
		return texts.ProcessDocuments
	}
	return processID
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

func (store *LiveStore) List_VisibleDocuments(ctx context.Context, token, scopeFilter, processFilter string) ([]DocumentLine, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if scopeFilter == texts.ScopeAll && !account.Has(texts.PermissionPersonalViewAll) {
		return nil, ErrAccessDenied
	}
	visibleSQL := global_VisibleSQL("$3")
	rows, err := store.pool.Query(ctx, `
		SELECT d.document_id, d.title, d.edition, d.access_level, d.scope, d.owner_id,
			COALESCE(u.display_name, ''), d.uploaded_at, d.process_id, d.visible_to_all,
			COALESCE((SELECT array_agg(dr.role_id ORDER BY dr.role_id) FROM document_roles dr WHERE dr.document_id = d.document_id), '{}')
		FROM documents d
		LEFT JOIN users u ON u.user_id = d.owner_id AND d.owner_id <> ''
		WHERE
			(
				($2 = 'all' AND (d.scope = 'personal' OR (d.scope = 'global' AND `+visibleSQL+`)))
				OR ($2 = 'global' AND d.scope = 'global' AND `+visibleSQL+`)
				OR ($2 = 'personal' AND d.scope = 'personal' AND d.owner_id = $1)
				OR ($2 = '' AND (
					(d.scope = 'global' AND `+visibleSQL+`)
					OR (d.scope = 'personal' AND d.owner_id = $1)
				))
			)
			AND ($4 = '' OR d.process_id = $4)
		ORDER BY d.document_id`, account.UserID, scopeFilter, role_Filter(account.RoleIDs), processFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := []DocumentLine{}
	for rows.Next() {
		var line DocumentLine
		if err := rows.Scan(&line.DocumentID, &line.Title, &line.Edition, &line.AccessLevel, &line.Scope, &line.OwnerID, &line.OwnerName, &line.UploadedAt, &line.ProcessID, &line.VisibleToAll, &line.RoleIDs); err != nil {
			return nil, err
		}
		if line.RoleIDs == nil {
			line.RoleIDs = []string{}
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
	var scope string
	var ownerID string
	var visibleToAll bool
	var documentRoleIDs []string
	err = store.pool.QueryRow(ctx, `
		SELECT scope, owner_id, visible_to_all,
			COALESCE((SELECT array_agg(role_id) FROM document_roles WHERE document_id = $1), '{}'),
			file_name, content_type, body
		FROM documents WHERE document_id = $1`, documentID).Scan(&scope, &ownerID, &visibleToAll, &documentRoleIDs, &file.FileName, &file.ContentType, &file.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return DocumentFile{}, ErrNotFound
	}
	if err != nil {
		return DocumentFile{}, err
	}
	if status := document_ReadError(account, scope, ownerID, visibleToAll, documentRoleIDs); status != nil {
		return DocumentFile{}, status
	}
	return file, nil
}

func (store *LiveStore) Delete_Document(ctx context.Context, token, documentID string) error {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return err
	}
	var scope string
	var ownerID string
	var visibleToAll bool
	var documentRoleIDs []string
	err = store.pool.QueryRow(ctx, `
		SELECT scope, owner_id, visible_to_all,
			COALESCE((SELECT array_agg(role_id) FROM document_roles WHERE document_id = $1), '{}')
		FROM documents WHERE document_id = $1`, documentID).Scan(&scope, &ownerID, &visibleToAll, &documentRoleIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status := document_ReadError(account, scope, ownerID, visibleToAll, documentRoleIDs); status != nil {
		return status
	}
	if !document_CanDelete(account, scope, ownerID) {
		return ErrAccessDenied
	}
	_, err = store.pool.Exec(ctx, `DELETE FROM documents WHERE document_id = $1`, documentID)
	return err
}

func document_ReadError(account Account, scope, ownerID string, visibleToAll bool, documentRoleIDs []string) error {
	if scope == texts.ScopePersonal {
		if account.UserID == ownerID || account.Has(texts.PermissionPersonalViewAll) {
			return nil
		}
		return ErrNotFound
	}
	if auth.Sees_GlobalDocument(account.RoleIDs, visibleToAll, documentRoleIDs) {
		return nil
	}
	return ErrAccessDenied
}

func document_CanDelete(account Account, scope, ownerID string) bool {
	if scope == texts.ScopePersonal {
		return account.UserID == ownerID || account.Has(texts.PermissionPersonalViewAll)
	}
	return account.Has(texts.PermissionGlobalManage)
}

func global_VisibleSQL(roleParam string) string {
	return `(d.visible_to_all OR EXISTS (
		SELECT 1 FROM document_roles dr
		WHERE dr.document_id = d.document_id AND dr.role_id = ANY(` + roleParam + `::text[])))`
}

// Чужой личный документ не попадает в поиск, даже при documents.personal.view_all: этот флаг открывает список и удаление
func (store *LiveStore) Find_ChunksBySimilarity(ctx context.Context, embedding []float32, userID string, roleIDs []string, processID string, limit int) ([]SimilarChunk, error) {
	return store.query_Chunks(ctx, embedding, limit, `
		SELECT d.document_id, d.title, d.edition, c.section, c.paragraph, c.appendix, c.content, d.scope,
			c.embedding <=> $1::vector AS distance
		FROM chunks c
		JOIN documents d ON d.document_id = c.document_id
		WHERE ((d.scope = 'global' AND `+global_VisibleSQL("$2")+`)
			OR (d.scope = 'personal' AND d.owner_id = $3))
			AND NOT `+contacts.Title_SQLCondition()+`
			AND ($4 = '' OR d.process_id = $4)
		ORDER BY c.embedding <=> $1::vector
		LIMIT $5`, role_Filter(roleIDs), userID, processID)
}

func (store *LiveStore) Find_ClosedGlobalChunks(ctx context.Context, embedding []float32, roleIDs []string, processID string, limit int) ([]SimilarChunk, error) {
	return store.query_Chunks(ctx, embedding, limit, `
		SELECT d.document_id, d.title, d.edition, c.section, c.paragraph, c.appendix, c.content, d.scope,
			c.embedding <=> $1::vector AS distance
		FROM chunks c
		JOIN documents d ON d.document_id = c.document_id
		WHERE d.scope = 'global' AND NOT d.visible_to_all
			AND NOT EXISTS (
				SELECT 1 FROM document_roles dr
				WHERE dr.document_id = d.document_id AND dr.role_id = ANY($2::text[])
			)
			AND NOT `+contacts.Title_SQLCondition()+`
			AND ($3 = '' OR d.process_id = $3)
		ORDER BY c.embedding <=> $1::vector
		LIMIT $4`, role_Filter(roleIDs), processID)
}

func (store *LiveStore) Find_ContactChunk(ctx context.Context, embedding []float32, userID string, roleIDs []string, processID string) (SimilarChunk, bool, error) {
	found, err := store.query_Chunks(ctx, embedding, 1, `
		SELECT d.document_id, d.title, d.edition, c.section, c.paragraph, c.appendix, c.content, d.scope,
			c.embedding <=> $1::vector AS distance
		FROM chunks c
		JOIN documents d ON d.document_id = c.document_id
		WHERE ((d.scope = 'global' AND `+global_VisibleSQL("$2")+`)
			OR (d.scope = 'personal' AND d.owner_id = $3))
			AND `+contacts.Title_SQLCondition()+`
			AND ($4 = '' OR d.process_id = $4)
		ORDER BY c.embedding <=> $1::vector
		LIMIT $5`, role_Filter(roleIDs), userID, processID)
	if err != nil {
		return SimilarChunk{}, false, err
	}
	if len(found) == 0 || similarity_OfChunk(found[0]) < minReplySimilarity {
		return SimilarChunk{}, false, nil
	}
	return found[0], true, nil
}

func (store *LiveStore) query_Chunks(ctx context.Context, embedding []float32, limit int, query string, args ...any) ([]SimilarChunk, error) {
	if len(embedding) != store.dimensions {
		return nil, ErrEmbedSize
	}
	if limit <= 0 {
		limit = 5
	}
	queryArgs := []any{format_Vector(embedding)}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, limit)
	rows, err := store.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := []SimilarChunk{}
	for rows.Next() {
		var chunkRow SimilarChunk
		if err := rows.Scan(&chunkRow.DocumentID, &chunkRow.Title, &chunkRow.Edition, &chunkRow.Section, &chunkRow.Paragraph, &chunkRow.Appendix, &chunkRow.Content, &chunkRow.Scope, &chunkRow.Distance); err != nil {
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
