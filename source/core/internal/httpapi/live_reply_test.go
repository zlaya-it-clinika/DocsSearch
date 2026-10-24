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

	denied, deniedDocs, deniedErr := build_LiveAssistantReply(chatter, "вопрос", nil, nil, []livestore.SimilarChunk{closed}, defaultReplyPrompts())
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
	}}, nil, defaultReplyPrompts())
	if fallbackErr != nil || chatter.calls != 0 || fallback.Kind != texts.KindFallback || fallback.Text != texts.FallbackText || fallback.Contacts == nil {
		t.Fatalf("fallback %+v %v calls %d", fallback, fallbackErr, chatter.calls)
	}

	chatter.replies = []string{"1", "Ответ по фрагменту."}
	answer, answerDocs, answerErr := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{visible}, []livestore.SimilarChunk{closed}, defaultReplyPrompts())
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
	answer, docs, err := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{first, second}, nil, defaultReplyPrompts())
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
	fallback, _, fallbackErr := build_LiveAssistantReply(empty, "вопрос", nil, []livestore.SimilarChunk{first}, nil, defaultReplyPrompts())
	if fallbackErr != nil || empty.calls != 1 || fallback.Kind != texts.KindFallback || len(fallback.Citations) != 0 {
		t.Fatalf("empty select %+v calls %d err %v", fallback, empty.calls, fallbackErr)
	}

	broken := &scriptedChat{err: livestore.ErrEmbedUnavailable}
	_, _, brokenErr := build_LiveAssistantReply(broken, "вопрос", nil, []livestore.SimilarChunk{first}, nil, defaultReplyPrompts())
	if brokenErr == nil {
		t.Fatal("select error was ignored")
	}
}

func TestBuild_LiveAssistantReplyUsesProcessAnswerPrompt(t *testing.T) {
	prompts := defaultReplyPrompts()
	prompts.Answer = "промпт кадров"
	chatter := &scriptedChat{replies: []string{"1", "ответ кадров"}}
	chunk := livestore.SimilarChunk{DocumentID: "doc-a", Content: "текст кадров", Distance: 0.2}
	answer, _, err := build_LiveAssistantReply(chatter, "вопрос", nil, []livestore.SimilarChunk{chunk}, nil, prompts)
	if err != nil || answer.Text != "ответ кадров" || len(chatter.systems) != 2 || chatter.systems[1] != "промпт кадров" {
		t.Fatalf("answer %+v prompts %+v err %v", answer, chatter.systems, err)
	}
}

func TestBuild_DraftRevisionJoinsParts(t *testing.T) {
	left := strings.Repeat("а", 4000)
	right := strings.Repeat("б", 4000)
	chatter := &scriptedChat{replies: []string{
		`{"draftText":"первая","changes":[{"text":"Сократил начало","paragraph":"3.2"}]}`,
		"```json\n{\"draftText\":\"вторая\",\"changes\":[{\"text\":\"Убрал лишнее\",\"paragraph\":\"9.9\"}]}\n```",
	}}
	chunks := []livestore.SimilarChunk{{
		DocumentID: texts.SeedInstructionID,
		Title:      texts.SeedInstructionTitle,
		Edition:    "1.0",
		Section:    "3",
		Paragraph:  "3.2",
		Content:    "цитата пункта",
		Scope:      texts.ScopeGlobal,
	}}
	revised, changes, citations, err := build_DraftRevision(chatter, "сократи", chunks, left+"\n\n"+right)
	if err != nil {
		t.Fatal(err)
	}
	if revised != "первая\n\nвторая" || chatter.calls != 2 {
		t.Fatalf("%q calls %d", revised, chatter.calls)
	}
	if !strings.Contains(chatter.users[0], left) || strings.Contains(chatter.users[0], right) {
		t.Fatal("parts were mixed")
	}
	if chatter.systems[0] != texts.LiveDraftPrompt {
		t.Fatalf("prompt %q", chatter.systems[0])
	}
	if len(changes) != 2 || changes[0].Citation == nil || changes[0].Citation.Paragraph != "3.2" || changes[0].Citation.DocumentID != texts.SeedInstructionID || changes[0].Citation.Quote != "цитата пункта" {
		t.Fatalf("changes %+v", changes)
	}
	if changes[1].Citation != nil {
		t.Fatal("invented citation")
	}
	if len(citations) != 1 || citations[0].Paragraph != "3.2" {
		t.Fatalf("citations %+v", citations)
	}
	keptChat := &scriptedChat{replies: []string{`{"draftText":"тот же","changes":[]}`}}
	kept, keptChanges, keptCitations, keptErr := build_DraftRevision(keptChat, "проверь", nil, "короткий")
	if keptErr != nil || kept != "тот же" || len(keptChanges) != 0 || len(keptCitations) != 0 || draft_Summary(keptChanges) != texts.DraftRevisionKept {
		t.Fatalf("kept %q %+v %v", kept, keptChanges, keptErr)
	}
	bad := &scriptedChat{replies: []string{"нет"}}
	if _, _, _, badErr := build_DraftRevision(bad, "проверь", nil, "короткий"); badErr == nil {
		t.Fatal("bad json accepted")
	}
}

func defaultReplyPrompts() replyPrompts {
	return replyPrompts{Select: texts.LiveSelectPrompt, Answer: texts.LiveAnswerPrompt, Clarify: texts.LiveClarifyPrompt}
}
