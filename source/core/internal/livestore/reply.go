package livestore

import (
	"context"
	"math"

	"docssearch/core/internal/texts"
)

const (
	minReplySimilarity   = 0.4
	clarifySimilarityGap = 0.05
	replyChunkLimit      = 5
)

type ReplyCitation struct {
	DocumentID   string
	Document     string
	Edition      string
	Section      string
	Paragraph    string
	Appendix     string
	Quote        string
	OriginalLink string
}

func (store *LiveStore) Embed_Question(text string) ([]float32, error) {
	vectors, err := store.embed_Texts([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, ErrEmbedUnavailable
	}
	return vectors[0], nil
}

func (store *LiveStore) Find_ReplyChunks(ctx context.Context, embedding []float32, accessLevel int) ([]SimilarChunk, error) {
	return store.Find_ChunksBySimilarity(ctx, embedding, accessLevel, replyChunkLimit)
}

func similarity_OfChunk(chunk SimilarChunk) float64 {
	return 1 - chunk.Distance
}

// Нет фрагмента с близостью от 0.4 - fallback, модель не нужна.
// Два близких фрагмента из разных документов, разрыв близости меньше 0.05 - clarify.
// Иначе answer по фрагментам, которые прошли порог.
func Choose_LiveReplyKind(chunks []SimilarChunk) (string, []SimilarChunk) {
	selected := make([]SimilarChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if similarity_OfChunk(chunk) >= minReplySimilarity {
			selected = append(selected, chunk)
		}
	}
	if len(selected) == 0 {
		return texts.KindFallback, nil
	}
	if len(selected) >= 2 && selected[0].DocumentID != selected[1].DocumentID {
		gap := similarity_OfChunk(selected[0]) - similarity_OfChunk(selected[1])
		if gap < clarifySimilarityGap {
			return texts.KindClarify, selected
		}
	}
	return texts.KindAnswer, selected
}

func Relevance_Percent(chunk SimilarChunk) int {
	percent := int(math.Round(similarity_OfChunk(chunk) * 100))
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func Build_ReplyCitations(chunks []SimilarChunk) []ReplyCitation {
	citations := make([]ReplyCitation, 0, len(chunks))
	for _, chunk := range chunks {
		citations = append(citations, ReplyCitation{
			DocumentID:   chunk.DocumentID,
			Document:     chunk.Title,
			Edition:      chunk.Edition,
			Section:      chunk.Section,
			Paragraph:    chunk.Paragraph,
			Appendix:     chunk.Appendix,
			Quote:        chunk.Content,
			OriginalLink: original_Link(chunk.DocumentID, chunk.Paragraph),
		})
	}
	return citations
}

func original_Link(documentID, paragraph string) string {
	link := "/documents/" + documentID + "/file"
	if paragraph != "" {
		link += "#p" + paragraph
	}
	return link
}
