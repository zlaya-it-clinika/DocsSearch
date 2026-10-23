package httpapi

import (
	"strings"
	"testing"

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
