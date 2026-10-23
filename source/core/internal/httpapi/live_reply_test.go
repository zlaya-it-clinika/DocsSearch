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

type scriptedChat struct {
	replies []string
	calls   int
	systems []string
	users   []string
	err     error
}

func (chat *scriptedChat) Complete_Chat(messages []ollama.ChatMessage) (string, error) {
	chat.calls++
	if len(messages) > 0 {
		chat.systems = append(chat.systems, messages[0].Content)
		chat.users = append(chat.users, messages[len(messages)-1].Content)
	}
	if chat.err != nil {
		return "", chat.err
	}
	if chat.calls > len(chat.replies) {
		return "", livestore.ErrEmbedUnavailable
	}
	return chat.replies[chat.calls-1], nil
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
	chatter := &scriptedChat{}

	denied, deniedDocs, deniedErr := build_LiveAssistantReply(chatter, "вопрос", nil, nil, []livestore.SimilarChunk{closed})
	if deniedErr != nil || chatter.calls != 0 || denied.Kind != texts.KindAccessDenied || denied.Text != texts.AccessDeniedText {
		t.Fatalf("denied %+v %v calls %d", denied, deniedErr, chatter.calls)
	}
	if denied.RelevancePercent != 0 || len(denied.Citations) != 0 || len(deniedDocs) != 0 || denied.Contacts != nil {
		t.Fatalf("denied payload %+v %+v", denied, deniedDocs)
	}

	fallback, _, fallbackErr := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{{
		DocumentID: "doc-weak",
		Content:    "слабый",
		Distance:   0.8,
	}}, nil)
	if fallbackErr != nil || chatter.calls != 0 || fallback.Kind != texts.KindFallback || fallback.Text != texts.FallbackText || fallback.Contacts == nil {
		t.Fatalf("fallback %+v %v calls %d", fallback, fallbackErr, chatter.calls)
	}

	chatter.replies = []string{"1", "Ответ по фрагменту."}
	answer, answerDocs, answerErr := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{visible}, []livestore.SimilarChunk{closed})
	if answerErr != nil || chatter.calls != 2 || answer.Kind != texts.KindAnswer || answer.Text != "Ответ по фрагменту." {
		t.Fatalf("answer %+v %v calls %d", answer, answerErr, chatter.calls)
	}
	if chatter.systems[0] != texts.LiveSelectPrompt || chatter.systems[1] != texts.LiveAnswerPrompt {
		t.Fatalf("prompts %+v", chatter.systems)
	}
	if answer.RelevancePercent != 80 || len(answer.Citations) != 1 || answer.Citations[0].DocumentID != "doc-open" || answer.Citations[0].Scope != texts.ScopeGlobal || answer.Citations[0].Quote != "видимый фрагмент" {
		t.Fatalf("answer citation %+v", answer.Citations)
	}
	if len(answerDocs) != 1 || answerDocs[0].DocumentID != "doc-open" || answerDocs[0].Scope != texts.ScopeGlobal || answer.Contacts == nil {
		t.Fatalf("shown %+v contacts %+v", answerDocs, answer.Contacts)
	}
}

func TestGroundedSelectionUsesOnlyChosenFragments(t *testing.T) {
	first := livestore.SimilarChunk{DocumentID: "doc-a", Paragraph: "1.1", Content: "первый фрагмент", Distance: 0.2}
	second := livestore.SimilarChunk{DocumentID: "doc-a", Paragraph: "1.2", Content: "второй фрагмент", Distance: 0.3}
	chatter := &scriptedChat{replies: []string{"2", "Коротко по второму."}}
	answer, docs, err := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{first, second}, nil)
	if err != nil || answer.Kind != texts.KindAnswer || answer.Text != "Коротко по второму." || answer.RelevancePercent != 70 {
		t.Fatalf("answer %+v %v", answer, err)
	}
	if len(answer.Citations) != 1 || answer.Citations[0].Paragraph != "1.2" || answer.Citations[0].Quote != "второй фрагмент" || len(docs) != 1 {
		t.Fatalf("citation %+v docs %+v", answer.Citations, docs)
	}
	if strings.Contains(chatter.users[1], "первый фрагмент") || !strings.Contains(chatter.users[1], "второй фрагмент") {
		t.Fatalf("second prompt %s", chatter.users[1])
	}

	empty := &scriptedChat{replies: []string{"нет"}}
	fallback, _, fallbackErr := build_LiveAssistantReply(empty, "вопрос", nil, []livestore.SimilarChunk{first}, nil)
	if fallbackErr != nil || empty.calls != 1 || fallback.Kind != texts.KindFallback || len(fallback.Citations) != 0 {
		t.Fatalf("empty select %+v calls %d err %v", fallback, empty.calls, fallbackErr)
	}

	broken := &scriptedChat{err: livestore.ErrEmbedUnavailable}
	_, _, brokenErr := build_LiveAssistantReply(broken, "вопрос", nil, []livestore.SimilarChunk{first}, nil)
	if brokenErr == nil {
		t.Fatal("select error was ignored")
	}
}
