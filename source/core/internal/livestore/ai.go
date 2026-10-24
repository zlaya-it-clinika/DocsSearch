package livestore

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

// Раздел app_settings с настройками локальной модели и отметка о выдаче флага ai.manage старым ролям
const (
	settingsKeyAI           = "ai"
	settingsKeyAIPermission = "permission_ai_manage"
)

// Сколько фрагментов отправлять в модель эмбеддингов за раз при переиндексации
const reindexBatchSize = 16

var ErrReindexRunning = errors.New("reindex running")

type ReindexStatus struct {
	Running    bool
	Done       int
	Total      int
	Failed     bool
	FinishedAt time.Time
}

type reindexState struct {
	mu     sync.Mutex
	status ReindexStatus
}

func (store *LiveStore) Require_Permission(ctx context.Context, token, permission string) error {
	_, err := store.require_Permission(ctx, token, permission)
	return err
}

func (store *LiveStore) Dimensions() int {
	return store.dimensions
}

// Сохранённые в админ-панели настройки. false - их ещё не сохраняли, действуют значения из .env
func (store *LiveStore) Read_SavedAISettings(ctx context.Context) (ollama.Settings, bool, error) {
	var exists bool
	if err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_settings WHERE key = $1)`, settingsKeyAI).Scan(&exists); err != nil || !exists {
		return ollama.Settings{}, false, err
	}
	var settings ollama.Settings
	if err := store.read_Setting(ctx, settingsKeyAI, &settings); err != nil {
		return ollama.Settings{}, false, err
	}
	return settings, true, nil
}

func (store *LiveStore) Save_AISettings(ctx context.Context, token string, settings ollama.Settings) error {
	if _, err := store.require_Permission(ctx, token, texts.PermissionAIManage); err != nil {
		return err
	}
	return store.write_Setting(ctx, settingsKeyAI, settings)
}

// Флаг ai.manage появился после первых установок: один раз выдаём его главному администратору и администратору.
// Дальше набор флагов роли меняет только конструктор ролей
func (store *LiveStore) grant_AIPermission(ctx context.Context) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var granted bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_settings WHERE key = $1)`, settingsKeyAIPermission).Scan(&granted); err != nil {
		return err
	}
	if granted {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission)
		SELECT role_id, $1 FROM roles WHERE role_id IN ($2, $3)
		ON CONFLICT DO NOTHING`, texts.PermissionAIManage, texts.RoleIDChief, texts.RoleIDAdmin); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app_settings (key, value) VALUES ($1, 'true'::jsonb) ON CONFLICT (key) DO NOTHING`, settingsKeyAIPermission); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *LiveStore) Read_ReindexStatus(ctx context.Context, token string) (ReindexStatus, error) {
	if _, err := store.require_Permission(ctx, token, texts.PermissionAIManage); err != nil {
		return ReindexStatus{}, err
	}
	store.reindex.mu.Lock()
	defer store.reindex.mu.Unlock()
	return store.reindex.status, nil
}

// Пересчёт векторов всех фрагментов текущей моделью эмбеддингов. Идёт в фоне, ход - в Read_ReindexStatus
func (store *LiveStore) Start_Reindex(ctx context.Context, token string) (ReindexStatus, error) {
	if _, err := store.require_Permission(ctx, token, texts.PermissionAIManage); err != nil {
		return ReindexStatus{}, err
	}
	store.reindex.mu.Lock()
	defer store.reindex.mu.Unlock()
	if store.reindex.status.Running {
		return store.reindex.status, ErrReindexRunning
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM chunks`).Scan(&total); err != nil {
		return ReindexStatus{}, err
	}
	store.reindex.status = ReindexStatus{Running: true, Total: total}
	go store.run_Reindex()
	return store.reindex.status, nil
}

type reindexChunk struct {
	id      int64
	content string
}

func (store *LiveStore) run_Reindex() {
	ctx := context.Background()
	failed := store.reindex_Chunks(ctx) != nil
	store.reindex.mu.Lock()
	defer store.reindex.mu.Unlock()
	store.reindex.status.Running = false
	store.reindex.status.Failed = failed
	store.reindex.status.FinishedAt = time.Now().UTC()
}

func (store *LiveStore) reindex_Chunks(ctx context.Context) error {
	rows, err := store.pool.Query(ctx, `SELECT chunk_id, content FROM chunks ORDER BY chunk_id`)
	if err != nil {
		return err
	}
	chunks := []reindexChunk{}
	for rows.Next() {
		var chunk reindexChunk
		if err := rows.Scan(&chunk.id, &chunk.content); err != nil {
			rows.Close()
			return err
		}
		chunks = append(chunks, chunk)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for start := 0; start < len(chunks); start += reindexBatchSize {
		batch := chunks[start:min(start+reindexBatchSize, len(chunks))]
		contents := make([]string, len(batch))
		for index, chunk := range batch {
			contents[index] = chunk.content
		}
		vectors, err := store.embed_Texts(contents)
		if err != nil {
			log.Printf("reindex error: %v", err)
			return err
		}
		for index, chunk := range batch {
			if _, err := store.pool.Exec(ctx, `UPDATE chunks SET embedding = $2::vector WHERE chunk_id = $1`, chunk.id, format_Vector(vectors[index])); err != nil {
				return err
			}
		}
		store.reindex.mu.Lock()
		store.reindex.status.Done = start + len(batch)
		store.reindex.mu.Unlock()
	}
	return nil
}
