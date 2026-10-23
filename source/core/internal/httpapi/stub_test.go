package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"docssearch/core/internal/config"
	"docssearch/core/internal/texts"
)

func TestStubDialogDocumentsRolesAndAudit(t *testing.T) {
	handler := must_Handler(t, config.Config{Mode: config.ModeStub})
	employeeToken := login(t, handler, texts.SeedEmployeeLogin, texts.SeedEmployeePassword)
	adminToken := login(t, handler, texts.SeedAdminLogin, texts.SeedAdminPassword)

	me := doJSON(t, handler, http.MethodGet, "/me", employeeToken, nil)
	if me.Code != http.StatusOK {
		t.Fatalf("me status %d", me.Code)
	}
	profile := decodeJSON[profileResponse](t, me)
	if profile.UserID != texts.SeedEmployeeID || profile.AccessLevel != texts.SeedEmployeeAccessLevel || profile.DisplayName != texts.SeedEmployeeDisplayName || profile.Role != texts.RoleUser {
		t.Fatalf("profile %+v", profile)
	}

	sessionID := createSession(t, handler, employeeToken)
	answer := postMessage(t, handler, employeeToken, sessionID, "Как зарегистрировать входящий документ?")
	if answer.Kind != texts.KindAnswer || answer.RelevancePercent != texts.AnswerRelevancePercent || len(answer.Citations) != 1 {
		t.Fatalf("answer %+v", answer)
	}
	if answer.Citations[0].OriginalLink != "/documents/doc-1/file#p3.2" {
		t.Fatalf("link %s", answer.Citations[0].OriginalLink)
	}
	if answer.Role != "" {
		t.Fatalf("post message should omit role, got %s", answer.Role)
	}
	listed := doJSON(t, handler, http.MethodGet, "/sessions", employeeToken, nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("sessions %d %s", listed.Code, listed.Body.String())
	}
	var sessionList struct {
		Sessions []sessionLine `json:"sessions"`
	}
	decodeInto(t, listed, &sessionList)
	if len(sessionList.Sessions) != 1 || sessionList.Sessions[0].SessionID != sessionID || sessionList.Sessions[0].Title == "" {
		t.Fatalf("session list %+v", sessionList.Sessions)
	}

	clarify := postMessage(t, handler, employeeToken, sessionID, "Уточни тип документа")
	if clarify.Kind != texts.KindClarify || len(clarify.Citations) != 0 || clarify.RelevancePercent != 0 {
		t.Fatalf("clarify %+v", clarify)
	}
	fallback := postMessage(t, handler, employeeToken, sessionID, "Этого не найдено")
	if fallback.Kind != texts.KindFallback || len(fallback.Citations) != 0 {
		t.Fatalf("fallback %+v", fallback)
	}
	denied := postMessage(t, handler, employeeToken, sessionID, "Нет доступа к регламенту")
	if denied.Kind != texts.KindAccessDenied {
		t.Fatalf("denied %+v", denied)
	}

	templateSession := createSession(t, handler, employeeToken)
	field := postMessage(t, handler, employeeToken, templateSession, "Нужен шаблон служебной записки")
	if field.Kind != texts.KindTemplateField || field.Template == nil || field.Template.Field != texts.TemplateField {
		t.Fatalf("template field %+v", field)
	}
	ready := postMessage(t, handler, employeeToken, templateSession, "не найдено")
	if ready.Kind != texts.KindTemplateReady || ready.Template == nil || ready.Template.DraftText != texts.TemplateDraftText {
		t.Fatalf("template ready %+v", ready)
	}

	historyRec := doJSON(t, handler, http.MethodGet, "/sessions/"+sessionID, employeeToken, nil)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status %d", historyRec.Code)
	}
	var history struct {
		SessionID string `json:"sessionId"`
		Messages  []struct {
			Role string `json:"role"`
			Kind string `json:"kind"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	decodeInto(t, historyRec, &history)
	if history.SessionID != sessionID || len(history.Messages) != 8 || history.Messages[0].Role != "user" || history.Messages[1].Role != "assistant" {
		t.Fatalf("history %+v", history)
	}

	adminHistory := doJSON(t, handler, http.MethodGet, "/sessions/"+sessionID, adminToken, nil)
	if adminHistory.Code != http.StatusForbidden {
		t.Fatalf("foreign session %d", adminHistory.Code)
	}
	missing := doJSON(t, handler, http.MethodGet, "/sessions/session-missing", employeeToken, nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing session %d", missing.Code)
	}

	employeeDocs := listDocuments(t, handler, employeeToken)
	if len(employeeDocs) != 1 || employeeDocs[0].DocumentID != texts.SeedInstructionID {
		t.Fatalf("employee docs %+v", employeeDocs)
	}
	adminDocs := listDocuments(t, handler, adminToken)
	if len(adminDocs) != 2 {
		t.Fatalf("admin docs %+v", adminDocs)
	}

	openFile := doRaw(t, handler, http.MethodGet, "/documents/"+texts.SeedInstructionID+"/file", employeeToken, nil, "")
	if openFile.Code != http.StatusOK || openFile.Body.String() != texts.SeedInstructionBody {
		t.Fatalf("open file %d %s", openFile.Code, openFile.Body.String())
	}
	closedFile := doRaw(t, handler, http.MethodGet, "/documents/"+texts.SeedRestrictedID+"/file", employeeToken, nil, "")
	if closedFile.Code != http.StatusForbidden {
		t.Fatalf("closed file %d", closedFile.Code)
	}
	adminFile := doRaw(t, handler, http.MethodGet, "/documents/"+texts.SeedRestrictedID+"/file", adminToken, nil, "")
	if adminFile.Code != http.StatusOK || adminFile.Body.String() != texts.SeedRestrictedBody {
		t.Fatalf("admin file %d", adminFile.Code)
	}

	upload := uploadDocument(t, handler, employeeToken, "note.txt", "hello", "Новый регламент", "1.0", "1")
	if upload.Status != texts.StatusAccepted || upload.DocumentID == "" {
		t.Fatalf("upload %+v", upload)
	}
	if !hasDocument(listDocuments(t, handler, employeeToken), upload.DocumentID) {
		t.Fatal("uploaded document is missing from the list")
	}
	replaced := uploadDocument(t, handler, adminToken, "instruction-2.txt", "edition two", texts.SeedInstructionTitle, "2.0", "1")
	if replaced.DocumentID != texts.SeedInstructionID || replaced.Edition != "2.0" {
		t.Fatalf("replace %+v", replaced)
	}
	replacedFile := doRaw(t, handler, http.MethodGet, "/documents/"+texts.SeedInstructionID+"/file", employeeToken, nil, "")
	if replacedFile.Body.String() != "edition two" {
		t.Fatalf("replaced body %s", replacedFile.Body.String())
	}

	hidden := uploadDocument(t, handler, adminToken, "secret.txt", "secret", "Секрет", "1.0", "2")
	if hasDocument(listDocuments(t, handler, employeeToken), hidden.DocumentID) {
		t.Fatal("employee sees a closed upload")
	}
	if !hasDocument(listDocuments(t, handler, adminToken), hidden.DocumentID) {
		t.Fatal("admin does not see a closed upload")
	}

	employeeAudit := doJSON(t, handler, http.MethodGet, "/audit", employeeToken, nil)
	if employeeAudit.Code != http.StatusForbidden {
		t.Fatalf("employee audit %d", employeeAudit.Code)
	}
	auditRec := doJSON(t, handler, http.MethodGet, "/audit", adminToken, nil)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit status %d", auditRec.Code)
	}
	var auditBody struct {
		Entries []auditEntry `json:"entries"`
	}
	decodeInto(t, auditRec, &auditBody)
	if len(auditBody.Entries) == 0 || auditBody.Entries[0].Question != "Как зарегистрировать входящий документ?" {
		t.Fatalf("audit %+v", auditBody.Entries)
	}
	if len(auditBody.Entries[0].Documents) != 1 || auditBody.Entries[0].Documents[0].DocumentID != texts.SeedInstructionID {
		t.Fatalf("audit documents %+v", auditBody.Entries[0].Documents)
	}

	patched := doJSON(t, handler, http.MethodPatch, "/users/"+texts.SeedEmployeeID, adminToken, map[string]any{
		"displayName": "Сотрудник архива",
		"accessLevel": 2,
	})
	if patched.Code != http.StatusOK {
		t.Fatalf("patch status %d body %s", patched.Code, patched.Body.String())
	}
	var patchedUser accountResponse
	decodeInto(t, patched, &patchedUser)
	if patchedUser.DisplayName != "Сотрудник архива" || patchedUser.AccessLevel != 2 || patchedUser.Role != texts.RoleAdmin {
		t.Fatalf("patched %+v", patchedUser)
	}
	if !hasDocument(listDocuments(t, handler, employeeToken), texts.SeedRestrictedID) {
		t.Fatal("raised access level still hides the closed document")
	}

	created := doJSON(t, handler, http.MethodPost, "/users", adminToken, map[string]any{
		"login":       "clerk",
		"password":    "clerk",
		"displayName": "Делопроизводитель",
		"accessLevel": 1,
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create user %d %s", created.Code, created.Body.String())
	}
	duplicate := doJSON(t, handler, http.MethodPost, "/users", adminToken, map[string]any{
		"login":       "clerk",
		"password":    "other",
		"displayName": "Другой",
	})
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate login %d", duplicate.Code)
	}
	clerkToken := login(t, handler, "clerk", "clerk")
	if clerkToken == "" {
		t.Fatal("created user cannot login")
	}
	employeeUsers := doJSON(t, handler, http.MethodGet, "/users", clerkToken, nil)
	if employeeUsers.Code != http.StatusForbidden {
		t.Fatalf("clerk users %d", employeeUsers.Code)
	}

	draft := uploadDraft(t, handler, employeeToken)
	if draft.Code != http.StatusOK || !strings.Contains(draft.Body.String(), "/documents/doc-1/file#p4.1") {
		t.Fatalf("draft %d %s", draft.Code, draft.Body.String())
	}

	badPassword := doJSON(t, handler, http.MethodPost, "/auth/login", "", map[string]string{
		"login":    texts.SeedEmployeeLogin,
		"password": "wrong",
	})
	if badPassword.Code != http.StatusUnauthorized {
		t.Fatalf("bad password %d", badPassword.Code)
	}
	if employeeToken != "" && !strings.Contains(badPassword.Header().Get("Access-Control-Allow-Origin"), "*") {
		t.Fatal("missing CORS header")
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	handler := must_Handler(t, config.Config{Mode: config.ModeStub})
	token := login(t, handler, texts.SeedEmployeeLogin, texts.SeedEmployeePassword)
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout %d %s", response.Code, response.Body.String())
	}
	me := doJSON(t, handler, http.MethodGet, "/me", token, nil)
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("token still works %d", me.Code)
	}
}

func TestLiveModeRequiresDatabase(t *testing.T) {
	_, err := NewHandler(config.Config{Mode: config.ModeLive})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("live startup error %v", err)
	}
}

func TestDebugLogSkipsSecrets(t *testing.T) {
	var logs bytes.Buffer
	original := logWriter()
	setLogWriter(&logs)
	defer setLogWriter(original)

	handler := must_Handler(t, config.Config{Mode: config.ModeStub, Debug: true})
	response := doJSON(t, handler, http.MethodPost, "/auth/login", "", map[string]string{
		"login":    texts.SeedEmployeeLogin,
		"password": texts.SeedEmployeePassword,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("login %d", response.Code)
	}
	line := logs.String()
	if strings.Contains(line, texts.SeedEmployeePassword) || strings.Contains(line, "Bearer") {
		t.Fatalf("debug log leaked secrets: %s", line)
	}
	if !strings.Contains(line, "POST /auth/login 200") {
		t.Fatalf("debug log %s", line)
	}
}

func must_Handler(t *testing.T, loadedConfig config.Config) http.Handler {
	t.Helper()
	handler, err := NewHandler(loadedConfig)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func login(t *testing.T, handler http.Handler, loginName, password string) string {
	t.Helper()
	response := doJSON(t, handler, http.MethodPost, "/auth/login", "", map[string]string{
		"login":    loginName,
		"password": password,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("login %s status %d body %s", loginName, response.Code, response.Body.String())
	}
	body := decodeJSON[loginResponse](t, response)
	if body.Token == "" {
		t.Fatal("empty token")
	}
	return body.Token
}

func createSession(t *testing.T, handler http.Handler, token string) string {
	t.Helper()
	response := doJSON(t, handler, http.MethodPost, "/sessions", token, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("session status %d", response.Code)
	}
	var body struct {
		SessionID string `json:"sessionId"`
	}
	decodeInto(t, response, &body)
	if body.SessionID == "" {
		t.Fatal("empty session")
	}
	return body.SessionID
}

func postMessage(t *testing.T, handler http.Handler, token, sessionID, text string) assistantReply {
	t.Helper()
	response := doJSON(t, handler, http.MethodPost, "/sessions/"+sessionID+"/messages", token, map[string]string{"text": text})
	if response.Code != http.StatusOK {
		t.Fatalf("message status %d body %s", response.Code, response.Body.String())
	}
	return decodeJSON[assistantReply](t, response)
}

func listDocuments(t *testing.T, handler http.Handler, token string) []documentResponse {
	t.Helper()
	response := doJSON(t, handler, http.MethodGet, "/documents", token, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("documents status %d", response.Code)
	}
	var body struct {
		Documents []documentResponse `json:"documents"`
	}
	decodeInto(t, response, &body)
	return body.Documents
}

func hasDocument(documents []documentResponse, documentID string) bool {
	for _, document := range documents {
		if document.DocumentID == documentID {
			return true
		}
	}
	return false
}

func uploadDocument(t *testing.T, handler http.Handler, token, fileName, content, title, edition, accessLevel string) uploadResponse {
	t.Helper()
	response := uploadMultipart(t, handler, "/documents", token, fileName, content, map[string]string{
		"title":       title,
		"edition":     edition,
		"accessLevel": accessLevel,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("upload status %d body %s", response.Code, response.Body.String())
	}
	return decodeJSON[uploadResponse](t, response)
}

func uploadDraft(t *testing.T, handler http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	return uploadMultipart(t, handler, "/drafts/check", token, "draft.txt", "черновик", nil)
}

func uploadMultipart(t *testing.T, handler http.Handler, path, token, fileName, content string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(part, content); err != nil {
		t.Fatal(err)
	}
	for key, value := range fields {
		if err = writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return doRaw(t, handler, http.MethodPost, path, token, payload.Bytes(), writer.FormDataContentType())
}

func doJSON(t *testing.T, handler http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	contentType := ""
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		contentType = "application/json"
	}
	return doRaw(t, handler, method, path, token, raw, contentType)
}

func doRaw(t *testing.T, handler http.Handler, method, path, token string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeJSON[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	decodeInto(t, response, &out)
	return out
}

func decodeInto(t *testing.T, response *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode %s: %v", response.Body.String(), err)
	}
}

func logWriter() io.Writer {
	return log.Writer()
}

func setLogWriter(writer io.Writer) {
	log.SetOutput(writer)
}
