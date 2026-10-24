package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"docssearch/core/internal/config"
	"docssearch/core/internal/texts"
)

// «Чтение» по умолчанию: документ не начинается, «План» без вида документа предлагает список
func TestStubChatModes(t *testing.T) {
	handler := must_Handler(t, config.Config{Mode: config.ModeStub})
	employeeToken := login(t, handler, texts.SeedEmployeeLogin, texts.SeedEmployeePassword)
	sessionID := createSession(t, handler, employeeToken)

	read := postMessage(t, handler, employeeToken, sessionID, "Подготовь служебную записку о замене ноутбука")
	if read.Kind != texts.KindFallback || read.Template != nil || read.Hint != texts.ChatModePlanHint {
		t.Fatalf("read mode started a document %+v", read)
	}
	if response := doJSON(t, handler, http.MethodPost, "/sessions/"+sessionID+"/messages", employeeToken, map[string]string{"text": "привет", "mode": "write"}); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown mode %d", response.Code)
	}

	// В «Плане» вопрос «как составить» тоже начинает документ
	plan := postModeMessage(t, handler, employeeToken, sessionID, "Как составить служебную записку?", texts.ChatModePlan)
	if plan.Kind != texts.KindTemplateField || plan.Template == nil || plan.Template.ID != "service-note" || plan.Hint != "" {
		t.Fatalf("plan layout %+v", plan)
	}
	var history struct {
		Mode string `json:"mode"`
	}
	decodeInto(t, doJSON(t, handler, http.MethodGet, "/sessions/"+sessionID, employeeToken, nil), &history)
	if history.Mode != texts.ChatModePlan {
		t.Fatalf("history mode %q", history.Mode)
	}

	// Переход в «Чтение» забывает начатый документ: в «Плане» реплика уже не ответ на вопрос макета
	if reply := postModeMessage(t, handler, employeeToken, sessionID, "Иванову", texts.ChatModeRead); reply.Kind == texts.KindTemplateField {
		t.Fatalf("read continued the layout %+v", reply)
	}
	choice := postModeMessage(t, handler, employeeToken, sessionID, "Надо решить вопрос с ноутбуком", texts.ChatModePlan)
	// Бланки модулей в списке - по названию из свойств файла, без пропусков и номеров примечаний
	if choice.Kind != texts.KindTemplateChoice || !strings.HasPrefix(choice.Text, texts.PlanChoiceText) || !strings.Contains(choice.Text, "1. Служебная записка") ||
		!strings.Contains(choice.Text, "Модуль распоряжения о создании рабочей группы") || strings.Contains(choice.Text, "[") {
		t.Fatalf("choice %+v", choice)
	}
	// Непонятный ответ - список еще раз, номер - документ по исходной просьбе
	retry := postMessage(t, handler, employeeToken, sessionID, "не знаю")
	if retry.Kind != texts.KindTemplateChoice || !strings.HasPrefix(retry.Text, texts.PlanChoiceRetryText) {
		t.Fatalf("retry %+v", retry)
	}
	picked := postMessage(t, handler, employeeToken, sessionID, "1")
	if picked.Kind != texts.KindTemplateField || picked.Template == nil || picked.Template.ID != "service-note" {
		t.Fatalf("picked %+v", picked)
	}
	if cancelled := postMessage(t, handler, employeeToken, sessionID, "отмена"); cancelled.Text != texts.LayoutCancelledText {
		t.Fatalf("cancel %+v", cancelled)
	}
	// Название в ответ на список
	postMessage(t, handler, employeeToken, sessionID, "Надо решить вопрос с ноутбуком")
	if letter := postMessage(t, handler, employeeToken, sessionID, "письмо"); letter.Template == nil || letter.Template.ID != "letter" {
		t.Fatalf("letter by name %+v", letter)
	}
}

// Модель выбирает вид документа: ровно один код из списка, иначе - список на выбор
func TestClassify_PlanDocument(t *testing.T) {
	documents := list_PlanDocuments()
	chat := &scriptedChat{replies: []string{"letter", "Код: act.", "НЕТ", "letter или act", "contract"}}
	for _, want := range []string{"letter", "act", "", "", ""} {
		document, found := classify_PlanDocument(chat, documents, "надо попросить поставщика прислать счет")
		if found != (want != "") || document.ID != want {
			t.Fatalf("want %q got %q %v", want, document.ID, found)
		}
	}
	if !strings.Contains(chat.systems[0], "НЕТ") || !strings.Contains(chat.users[0], "service-note - Служебная записка") {
		t.Fatalf("prompt %q %q", chat.systems[0], chat.users[0])
	}
	if _, found := classify_PlanDocument(nil, documents, "любая просьба"); found {
		t.Fatal("no model, no choice")
	}
}
