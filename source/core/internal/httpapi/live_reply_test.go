package httpapi

import (
	"strings"
	"testing"

	"docssearch/core/internal/livestore"
	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

func TestBuild_ChatMessagesKeepsHistory(t *testing.T) {
	messages := build_ChatMessages(texts.LiveAnswerPrompt, "А какой срок?", []chatTurn{
		{Role: "user", Text: "Как регистрировать?"},
		{Role: "assistant", Text: "В день поступления."},
	}, nil)
	if len(messages) != 4 {
		t.Fatalf("messages %d", len(messages))
	}
	if messages[0].Role != "system" || messages[0].Content != texts.LiveAnswerPrompt {
		t.Fatalf("system %+v", messages[0])
	}
	if messages[1].Content != "Как регистрировать?" || messages[2].Content != "В день поступления." {
		t.Fatalf("history %+v %+v", messages[1], messages[2])
	}
	if !strings.Contains(messages[3].Content, "А какой срок?") {
		t.Fatalf("question %s", messages[3].Content)
	}
}

type fixedChat struct {
	text   string
	called bool
}

func (chat *fixedChat) Complete_Chat(messages []ollama.ChatMessage) (string, error) {
	chat.called = true
	if len(messages) == 0 {
		return "", livestore.ErrEmbedUnavailable
	}
	return chat.text, nil
}

func TestBuild_LiveAssistantReplyKinds(t *testing.T) {
	visible := livestore.SimilarChunk{
		DocumentID: "doc-open",
		Title:      "Открытый",
		Edition:    "1.0",
		Paragraph:  "1.1",
		Content:    "видимый фрагмент",
		Scope:      texts.ScopeGlobal,
		Distance:   0.2,
	}
	closed := livestore.SimilarChunk{
		DocumentID: "doc-closed",
		Title:      "Закрытый",
		Content:    "закрытый фрагмент",
		Scope:      texts.ScopeGlobal,
		Distance:   0.1,
	}
	chatter := &fixedChat{text: "Ответ по фрагменту."}

	denied, deniedDocs, deniedErr := build_LiveAssistantReply(chatter, "вопрос", nil, nil, []livestore.SimilarChunk{closed})
	if deniedErr != nil || chatter.called || denied.Kind != texts.KindAccessDenied || denied.Text != texts.AccessDeniedText {
		t.Fatalf("denied %+v %v called %v", denied, deniedErr, chatter.called)
	}
	if denied.RelevancePercent != 0 || len(denied.Citations) != 0 || len(deniedDocs) != 0 {
		t.Fatalf("denied payload %+v %+v", denied, deniedDocs)
	}

	fallback, _, fallbackErr := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{{
		DocumentID: "doc-weak",
		Content:    "слабый",
		Distance:   0.8,
	}}, nil)
	if fallbackErr != nil || chatter.called || fallback.Kind != texts.KindFallback || fallback.Text != texts.FallbackText {
		t.Fatalf("fallback %+v %v called %v", fallback, fallbackErr, chatter.called)
	}

	answer, answerDocs, answerErr := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{visible}, []livestore.SimilarChunk{closed})
	if answerErr != nil || !chatter.called || answer.Kind != texts.KindAnswer || answer.Text != chatter.text {
		t.Fatalf("answer %+v %v", answer, answerErr)
	}
	if answer.RelevancePercent != 80 || len(answer.Citations) != 1 || answer.Citations[0].DocumentID != "doc-open" || answer.Citations[0].Scope != texts.ScopeGlobal || answer.Citations[0].Quote != "видимый фрагмент" {
		t.Fatalf("answer citation %+v", answer.Citations)
	}
	if len(answerDocs) != 1 || answerDocs[0].DocumentID != "doc-open" || answerDocs[0].Scope != texts.ScopeGlobal {
		t.Fatalf("shown %+v", answerDocs)
	}
}
