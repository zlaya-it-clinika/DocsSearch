package livestore

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestFind_ChunksBySimilarityFiltersAccessLevel(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is empty")
	}
	store, err := Open_LiveStore(databaseURL, 768, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	ctx := context.Background()
	documentID := "doc-search-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	_, err = store.pool.Exec(ctx, `
		INSERT INTO documents (document_id, title, edition, access_level, file_name, content_type, body)
		VALUES ($1, $2, '1.0', 1, 'probe.txt', 'text/plain', $3)`,
		documentID, documentID, []byte("probe"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM documents WHERE document_id = $1`, documentID)
	})

	near := make([]float32, 768)
	near[0] = 1
	closed := make([]float32, 768)
	closed[0] = 1
	other := make([]float32, 768)
	other[1] = 1
	insert := `
		INSERT INTO chunks (document_id, section, paragraph, appendix, access_level, content, embedding)
		VALUES ($1, '', '', '', $2, $3, $4::vector)`
	if _, err := store.pool.Exec(ctx, insert, documentID, 1, "открытый фрагмент", format_Vector(near)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, insert, documentID, 2, "закрытый фрагмент", format_Vector(closed)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, insert, documentID, 1, "другой фрагмент", format_Vector(other)); err != nil {
		t.Fatal(err)
	}

	found, err := store.Find_ChunksBySimilarity(ctx, near, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	sawNear := false
	for _, row := range found {
		if row.Content == "закрытый фрагмент" {
			t.Fatalf("closed chunk is visible: %+v", found)
		}
		if row.DocumentID == documentID && row.Content == "открытый фрагмент" {
			sawNear = true
			if row.Distance > 0.01 {
				t.Fatalf("distance %v", row.Distance)
			}
		}
	}
	if !sawNear {
		t.Fatalf("near chunk missing: %+v", found)
	}
}
