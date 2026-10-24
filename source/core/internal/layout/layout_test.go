package layout

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"docssearch/core/internal/eval"
	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

var testNow = time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)

type scriptedChat struct {
	replies []string
	calls   int
}

func (chat *scriptedChat) Complete_Chat(messages []ollama.ChatMessage) (string, error) {
	chat.calls++
	if chat.calls > len(chat.replies) {
		return "", errors.New("no reply")
	}
	return chat.replies[chat.calls-1], nil
}

func test_PNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	picture.Set(0, 0, color.Black)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func test_Organization(t *testing.T) Organization {
	return Organization{
		Officials: []Official{
			{Position: "Генеральный директор", FullName: "Петров Андрей Борисович", Chief: true},
			{Position: "Руководитель ИТ-отдела", FullName: "Иванов И.И."},
		},
		Emblem: &Image{ContentType: "image/png", Body: test_PNG(t, 40, 50)},
		Stamp:  &Image{ContentType: "image/png", Body: test_PNG(t, 300, 100)},
	}
}

func TestMatch_RequestTellsOrdersFromQuestions(t *testing.T) {
	for text, id := range map[string]string{
		"Подготовь служебную записку о замене ноутбука":      "service-note",
		"нужна служебка на закупку ноутбуков":                "purchase-request",
		"Составь докладную о срыве поставки":                 "report-note",
		"напиши пояснительную записку":                       "explanatory-note",
		"Сделай письмо в АО Профиль о предоставлении данных": "letter",
		"Подготовь акт о списании оргтехники":                "act",
		"Нужен протокол совещания":                           "protocol",
	} {
		info, ok := Match_Request(text)
		if !ok || info.ID != id {
			t.Fatalf("%q -> %q %v", text, info.ID, ok)
		}
	}
	for _, text := range []string{
		"Как составить служебную записку?",
		"Нужно ли указывать наименование вида документа на письме?",
		"Актуальная редакция инструкции",
		"служебная записка",
	} {
		if info, ok := Match_Request(text); ok {
			t.Fatalf("%q started %s", text, info.ID)
		}
	}
	// Вопросы к базе знаний не запускают макет
	questions, err := eval.Load_Questions()
	if err != nil {
		t.Fatal(err)
	}
	for _, question := range questions {
		if info, ok := Match_Request(question.Question); ok {
			t.Fatalf("eval question %q started %s", question.Question, info.ID)
		}
	}
}

func TestParse_PersonSignature(t *testing.T) {
	for text, want := range map[string][2]string{
		"менеджер отдела продаж Петрова Мария Сергеевна": {"Менеджер отдела продаж", "М.С. Петрова"},
		"Руководитель ИТ-отдела Иванов И.И.":             {"Руководитель ИТ-отдела", "И.И. Иванов"},
		"Генеральный директор А.Б. Петров":               {"Генеральный директор", "А.Б. Петров"},
		"директор Департамента ДОУ Сидоров П. П.":        {"Директор Департамента ДОУ", "П.П. Сидоров"},
	} {
		parsed := parse_Person(text)
		if parsed.Position != want[0] || parsed.Signature != want[1] {
			t.Fatalf("%q -> %q %q", text, parsed.Position, parsed.Signature)
		}
	}
}

func TestServiceNoteDialogWithoutModel(t *testing.T) {
	organization := test_Organization(t)
	info, _ := Match_Request("Подготовь служебную записку о замене ноутбука")
	step := Start_Layout(info, "Подготовь служебную записку о замене ноутбука", organization, nil, testNow)
	if step.Ready || step.Field != texts.LayoutFieldAddressee || !strings.Contains(step.Question, "2. Руководитель ИТ-отдела Иванов И.И.") {
		t.Fatalf("first step %+v", step)
	}
	if step.State.Answers[texts.LayoutFieldSubject] != "о замене ноутбука" {
		t.Fatalf("subject from request %+v", step.State.Answers)
	}
	// Фамилия в дательном падеже находит должностное лицо из списка
	step = Advance_Layout(step.State, "Иванову", organization, nil, testNow)
	if step.Field != texts.LayoutFieldAuthor || step.State.Answers[texts.LayoutFieldAddressee] != "Руководитель ИТ-отдела Иванов И.И." {
		t.Fatalf("addressee %+v", step)
	}
	step = Advance_Layout(step.State, "", organization, nil, testNow)
	if step.Field != texts.LayoutFieldAuthor {
		t.Fatalf("empty required answer accepted %+v", step)
	}
	step = Advance_Layout(step.State, "менеджер отдела продаж Петрова Мария Сергеевна", organization, nil, testNow)
	step = Advance_Layout(step.State, "Ноутбук сломался 10.08.2026. Прошу выдать новый до 20.08.2026.", organization, nil, testNow)
	if step.Field != texts.LayoutFieldAttach {
		t.Fatalf("attachments %+v", step)
	}
	step = Advance_Layout(step.State, "нет", organization, nil, testNow)
	if !step.Ready || len(step.Docx) == 0 || step.FileName != "Служебная записка о замене ноутбука.docx" {
		t.Fatalf("ready %+v", step)
	}
	for _, want := range []string{"Руководитель ИТ-отдела Иванов И.И.", "от менеджер отдела продаж", "СЛУЖЕБНАЯ ЗАПИСКА", "15.08.2026 №", "О замене ноутбука", "Прошу выдать новый до 20.08.2026.", "Менеджер отдела продаж    М.С. Петрова"} {
		if !strings.Contains(step.DraftText, want) {
			t.Fatalf("draft has no %q:\n%s", want, step.DraftText)
		}
	}
	documentXML := read_DocxPart(t, step.Docx, "word/document.xml")
	if !strings.Contains(documentXML, `w:firstLine="709"`) || !strings.Contains(documentXML, `<w:jc w:val="both"/>`) || strings.Contains(documentXML, "<w:drawing>") {
		t.Fatal("note layout: indent, justify, no emblem")
	}
	if !strings.Contains(read_DocxPart(t, step.Docx, "word/styles.xml"), "Times New Roman") {
		t.Fatal("font")
	}
}

func TestLetterUsesModelAndOrganization(t *testing.T) {
	organization := test_Organization(t)
	info, _ := Find_Layout("letter")
	chat := &scriptedChat{replies: []string{
		`{"addressee":"Сидоров","subject":"о предоставлении данных"}`,
		"```json\n{\"addressee\":[\"Генеральному директору\",\"АО \\\"Профиль\\\"\",\"Сидорову П.П.\"],\"author\":[],\"title\":\"О предоставлении данных\",\"greeting\":\"Уважаемый Петр Петрович!\",\"text\":[\"Просим направить данные до 01.09.2026.\"]}\n```",
	}}
	step := Start_Layout(info, "Сделай письмо Сидорову о предоставлении данных", organization, chat, testNow)
	if step.Field != texts.LayoutFieldPoints || step.State.Answers[texts.LayoutFieldAddressee] != "Сидоров" {
		t.Fatalf("extracted %+v", step)
	}
	step = Advance_Layout(step.State, "Просим направить данные до 01.09.2026", organization, chat, testNow)
	if step.Field != texts.LayoutFieldSigner || !strings.Contains(step.Question, "подпишет Генеральный директор Петров Андрей Борисович") {
		t.Fatalf("signer question %+v", step)
	}
	step = Advance_Layout(step.State, "нет", organization, chat, testNow)
	step = Advance_Layout(step.State, "нет", organization, chat, testNow)
	step = Advance_Layout(step.State, "Петрова Мария Сергеевна, +7 (495) 123-45-67", organization, chat, testNow)
	if !step.Ready || chat.calls != 2 {
		t.Fatalf("letter %+v calls %d", step, chat.calls)
	}
	for _, want := range []string{"Генеральному директору", "Уважаемый Петр Петрович!", "С уважением,", "Генеральный директор    А.Б. Петров", "+7 (495) 123-45-67"} {
		if !strings.Contains(step.DraftText, want) {
			t.Fatalf("letter has no %q:\n%s", want, step.DraftText)
		}
	}
	if strings.Contains(step.DraftText, "ПИСЬМО") {
		t.Fatal("letter has a document kind title")
	}
	documentXML := read_DocxPart(t, step.Docx, "word/document.xml")
	if strings.Count(documentXML, "<w:drawing>") != 2 || !strings.Contains(documentXML, "<w:tbl>") {
		t.Fatal("letter blank: emblem, stamp and two columns")
	}
	if len(read_DocxPart(t, step.Docx, "word/media/image1.png")) == 0 || !strings.Contains(read_DocxPart(t, step.Docx, "word/_rels/document.xml.rels"), "media/image2.png") {
		t.Fatal("images in package")
	}
}

func TestComposeFallsBackOnInventedNumber(t *testing.T) {
	info, _ := Find_Layout("service-note")
	state := State{LayoutID: info.ID, Request: "служебка", Answers: map[string]string{
		texts.LayoutFieldSubject: "о замене ноутбука",
		texts.LayoutFieldPoints:  "Прошу заменить ноутбук.",
	}}
	chat := &scriptedChat{replies: []string{`{"addressee":[],"author":[],"title":"О замене ноутбука","greeting":"","text":["Прошу заменить ноутбук до 25.08.2026."]}`}}
	composed := compose_Parts(info, state, chat, "15.08.2026")
	if len(composed.Text) != 1 || composed.Text[0] != "Прошу заменить ноутбук." {
		t.Fatalf("invented date kept %+v", composed)
	}
}

func TestProtocolAndActSignatures(t *testing.T) {
	organization := test_Organization(t)
	protocol, _ := Find_Layout("protocol")
	state := State{LayoutID: protocol.ID, Answers: map[string]string{
		texts.LayoutFieldSubject:   "совещания по внедрению ИИС ДО",
		texts.LayoutFieldPlace:     "",
		texts.LayoutFieldChair:     "директор Департамента ДОУ Сидоров П.П.",
		texts.LayoutFieldSecretary: "специалист Кузнецова Анна Олеговна",
		texts.LayoutFieldAttendees: "Иванов И.И.; Петрова М.С.",
		texts.LayoutFieldAgenda:    "О сроках внедрения; О обучении",
		texts.LayoutFieldPoints:    "СЛУШАЛИ: Сидорова П.П. ПОСТАНОВИЛИ: начать обучение.",
	}}
	step := Advance_Layout(state, "", organization, nil, testNow)
	for _, want := range []string{"ПРОТОКОЛ", "Председатель - директор Департамента ДОУ Сидоров П.П.", "1. О сроках внедрения", "2. О обучении", "Председатель    П.П. Сидоров", "Секретарь    А.О. Кузнецова"} {
		if !strings.Contains(step.DraftText, want) {
			t.Fatalf("protocol has no %q:\n%s", want, step.DraftText)
		}
	}
	act, _ := Find_Layout("act")
	signatures := committee_Signatures("член комиссии Орлов А.А.; председатель комиссии Белов Б.Б.; Зайцев В.В.")
	if len(signatures) != 3 || signatures[0].Name != "Б.Б. Белов" || signatures[1].Position != texts.LayoutCommitteeSign || signatures[2].Name != "В.В. Зайцев" {
		t.Fatalf("committee %+v", signatures)
	}
	if act.Kind != texts.LayoutKindAct {
		t.Fatal("act kind")
	}
}

func TestAdvanceCancels(t *testing.T) {
	step := Advance_Layout(State{LayoutID: "service-note", Field: texts.LayoutFieldAuthor}, "отмена", Organization{}, nil, testNow)
	if !step.Cancelled {
		t.Fatalf("cancel %+v", step)
	}
}

func read_DocxPart(t *testing.T, body []byte, name string) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	t.Fatalf("no %s in docx", name)
	return ""
}

func TestTitleAndHeaderNormalization(t *testing.T) {
	for title, want := range map[string]string{
		"Об замене ноутбука":    "О замене ноутбука",
		"о утверждении порядка": "Об утверждении порядка",
		"О едином подходе":      "О едином подходе",
		"Запрос данных":         "Запрос данных",
	} {
		if got := fix_Preposition(title); got != want {
			t.Fatalf("%q -> %q", title, got)
		}
	}
	letter := normalize_HeaderLines([]string{"Генеральному директору Сидорову Петру Петровичу", `АО "Профиль"`}, true)
	if strings.Join(letter, "|") != `АО "Профиль"|Генеральному директору|Сидорову П.П.` {
		t.Fatalf("letter addressee %q", letter)
	}
	author := normalize_HeaderLines([]string{"от менеджера отдела продаж Петровой Марии Сергеевны"}, false)
	if strings.Join(author, "|") != "от менеджера отдела продаж|Петровой М.С." {
		t.Fatalf("author %q", author)
	}
	if kept := normalize_HeaderLines([]string{"Руководителю ИТ-отдела", "Иванову И.И."}, false); strings.Join(kept, "|") != "Руководителю ИТ-отдела|Иванову И.И." {
		t.Fatalf("kept %q", kept)
	}
	// Заголовок модели без «О» заменяется темой из ответов
	info, _ := Find_Layout("letter")
	state := State{LayoutID: info.ID, Answers: map[string]string{texts.LayoutFieldSubject: "о предоставлении данных", texts.LayoutFieldPoints: "Просим направить данные."}}
	chat := &scriptedChat{replies: []string{`{"addressee":[],"author":[],"title":"Запрос данных от Организации","greeting":"","text":["Просим направить данные."]}`}}
	if composed := compose_Parts(info, state, chat, "15.08.2026"); composed.Title != "О предоставлении данных" {
		t.Fatalf("title %q", composed.Title)
	}
}

func TestLetterFrameAndDepartment(t *testing.T) {
	text := strip_LetterFrame([]string{"Уважаемый Петр Петрович!", "Просим направить данные. С уважением,", "С уважением,"}, "Уважаемый Петр Петрович!")
	if strings.Join(text, "|") != "Просим направить данные." {
		t.Fatalf("letter text %q", text)
	}
	if !check_DepartmentInPosition("Начальник отдела информационных технологий", "Отдел информационных технологий") || check_DepartmentInPosition("Начальник отдела", "Отдел закупок") {
		t.Fatal("department in position")
	}
	official := Official{Position: "Начальник отдела закупок", FullName: "Богатая М.П.", Department: "Отдел закупок"}
	if describe_Official(official) != "Начальник отдела закупок Богатая М.П." {
		t.Fatalf("describe %q", describe_Official(official))
	}
	info, _ := Find_Layout("service-note")
	chat := &scriptedChat{replies: []string{`{"subject":"Подготовь служебную записку о замене ноутбука"}`}}
	if answers := extract_Answers(info, "Подготовь служебную записку о замене ноутбука", chat); answers[texts.LayoutFieldSubject] != "о замене ноутбука" {
		t.Fatalf("subject %q", answers[texts.LayoutFieldSubject])
	}
}

func TestPurchaseSubjectFromRequest(t *testing.T) {
	info, _ := Find_Layout("purchase-request")
	if answers := extract_Answers(info, "нужна служебка на закупку ноутбуков", nil); answers[texts.LayoutFieldSubject] != "о закупке ноутбуков" {
		t.Fatalf("purchase subject %q", answers[texts.LayoutFieldSubject])
	}
}

func TestShortRequestDoesNotFillPoints(t *testing.T) {
	info, _ := Find_Layout("purchase-request")
	chat := &scriptedChat{replies: []string{`{"points":"закупка ноутбуков"}`}}
	if answers := extract_Answers(info, "нужна служебка на закупку ноутбуков", chat); answers[texts.LayoutFieldPoints] != "" {
		t.Fatalf("points from a short request %q", answers[texts.LayoutFieldPoints])
	}
}
