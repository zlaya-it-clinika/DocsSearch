package livestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"docssearch/core/internal/texts"
)

const (
	counterUser     = "user"
	counterDocument = "document"
	counterSession  = "session"
	counterAudit    = "audit"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrAccessDenied       = errors.New("access denied")
	ErrNotFound           = errors.New("not found")
	ErrLoginTaken         = errors.New("login taken")
	ErrEmbedSize          = errors.New("embed size")
	ErrEmbedUnavailable   = errors.New("embed unavailable")
	ErrUnsupportedFile    = errors.New("unsupported file")
)

type Embedder interface {
	Embed_Texts(inputs []string) ([][]float32, error)
}

type Account struct {
	UserID      string
	Login       string
	DisplayName string
	AccessLevel int
}

type LoginResult struct {
	Token       string
	UserID      string
	DisplayName string
	AccessLevel int
}

type DocumentLine struct {
	DocumentID  string
	Title       string
	Edition     string
	AccessLevel int
	Scope       string
	OwnerID     string
	OwnerName   string
	UploadedAt  time.Time
}

type DocumentFile struct {
	FileName    string
	ContentType string
	Body        []byte
}

type SimilarChunk struct {
	DocumentID string
	Title      string
	Edition    string
	Section    string
	Paragraph  string
	Appendix   string
	Content    string
	Scope      string
	Distance   float64
}

type LiveStore struct {
	pool       *pgxpool.Pool
	dimensions int
	embedder   Embedder
}

func Open_LiveStore(databaseURL string, dimensions int, embedder Embedder) (*LiveStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New(texts.LiveDatabaseMissing)
	}
	if dimensions <= 0 {
		return nil, errors.New(texts.LiveEmbedDimensions)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, errors.New(texts.LiveDatabaseConnect)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New(texts.LiveDatabaseConnect)
	}
	store := &LiveStore{pool: pool, dimensions: dimensions, embedder: embedder}
	if err := store.apply_Schema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.seed_Users(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (store *LiveStore) Close() {
	store.pool.Close()
}

func (store *LiveStore) apply_Schema(ctx context.Context) error {
	schema, err := read_MigrationSQL()
	if err != nil {
		return err
	}
	schema = strings.ReplaceAll(schema, "vector(768)", fmt.Sprintf("vector(%d)", store.dimensions))
	_, err = store.pool.Exec(ctx, schema)
	return err
}

func read_MigrationSQL() (string, error) {
	candidates := []string{
		"migrations/001_init.sql",
		"source/core/migrations/001_init.sql",
		"../../migrations/001_init.sql",
	}
	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate)
		if err == nil {
			return string(raw), nil
		}
	}
	return "", errors.New("migrations/001_init.sql")
}

func (store *LiveStore) seed_Users(ctx context.Context) error {
	seeds := []struct {
		id          string
		login       string
		password    string
		displayName string
		accessLevel int
	}{
		{texts.SeedAdminID, texts.SeedAdminLogin, texts.SeedAdminPassword, texts.SeedAdminDisplayName, texts.SeedAdminAccessLevel},
		{texts.SeedEmployeeID, texts.SeedEmployeeLogin, texts.SeedEmployeePassword, texts.SeedEmployeeDisplayName, texts.SeedEmployeeAccessLevel},
	}
	for _, seed := range seeds {
		var exists bool
		if err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)`, seed.id).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(seed.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = store.pool.Exec(ctx, `
			INSERT INTO users (user_id, login, password_hash, display_name, access_level)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id) DO NOTHING`,
			seed.id, seed.login, string(hash), seed.displayName, seed.accessLevel)
		if err != nil {
			return err
		}
	}
	return nil
}

func (store *LiveStore) next_ID(ctx context.Context, name string) (int, error) {
	var value int
	err := store.pool.QueryRow(ctx, `
		UPDATE id_counters SET value = value + 1 WHERE name = $1 RETURNING value - 1`, name).Scan(&value)
	return value, err
}
