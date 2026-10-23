package livestore

import (
	"testing"

	"docssearch/core/internal/texts"
)

func chunk_ForReply(documentID string, distance float64) SimilarChunk {
	return SimilarChunk{
		DocumentID: documentID,
		Title:      documentID,
		Edition:    "1.0",
		Paragraph:  "3.2",
		Content:    "Текст фрагмента",
		Distance:   distance,
	}
}

func TestChoose_LiveReplyKind(t *testing.T) {
	kind, selected := Choose_LiveReplyKind([]SimilarChunk{chunk_ForReply("doc-a", 0.7)})
	if kind != texts.KindFallback || selected != nil {
		t.Fatalf("weak %+v %s", selected, kind)
	}

	kind, selected = Choose_LiveReplyKind([]SimilarChunk{chunk_ForReply("doc-a", 0.2)})
	if kind != texts.KindAnswer || len(selected) != 1 {
		t.Fatalf("single %+v %s", selected, kind)
	}
	if Relevance_Percent(selected[0]) != 80 {
		t.Fatalf("percent %d", Relevance_Percent(selected[0]))
	}

	kind, selected = Choose_LiveReplyKind([]SimilarChunk{
		chunk_ForReply("doc-a", 0.2),
		chunk_ForReply("doc-b", 0.23),
	})
	if kind != texts.KindClarify || len(selected) != 2 {
		t.Fatalf("clarify %+v %s", selected, kind)
	}

	kind, selected = Choose_LiveReplyKind([]SimilarChunk{
		chunk_ForReply("doc-a", 0.2),
		chunk_ForReply("doc-a", 0.22),
	})
	if kind != texts.KindAnswer || len(selected) != 2 {
		t.Fatalf("same document %+v %s", selected, kind)
	}

	kind, selected = Choose_LiveReplyKind([]SimilarChunk{
		chunk_ForReply("doc-a", 0.2),
		chunk_ForReply("doc-b", 0.3),
	})
	if kind != texts.KindAnswer || len(selected) != 2 {
		t.Fatalf("wide gap %+v %s", selected, kind)
	}

	citations := Build_ReplyCitations(selected[:1])
	if len(citations) != 1 || citations[0].OriginalLink != "/documents/doc-a/file#p3.2" || citations[0].Quote != "Текст фрагмента" {
		t.Fatalf("citation %+v", citations)
	}
}
