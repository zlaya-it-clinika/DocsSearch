package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"docssearch/core/internal/livestore"
	"docssearch/core/internal/ollama"
	"docssearch/core/internal/texts"
)

type liveAPI struct {
	store   *livestore.LiveStore
	chatter interface {
		Complete_Chat(messages []ollama.ChatMessage) (string, error)
	}
}

func register_LiveRoutes(store *livestore.LiveStore, chatter interface {
	Complete_Chat(messages []ollama.ChatMessage) (string, error)
}) *http.ServeMux {
	api := &liveAPI{store: store, chatter: chatter}
	routeMux := http.NewServeMux()
	routeMux.HandleFunc("POST /auth/login", api.serve_Login)
	routeMux.HandleFunc("POST /auth/logout", api.serve_Logout)
	routeMux.HandleFunc("GET /me", api.serve_CurrentUser)
	routeMux.HandleFunc("POST /sessions", api.serve_CreateSession)
	routeMux.HandleFunc("GET /sessions", api.serve_ListSessions)
	routeMux.HandleFunc("GET /sessions/{sessionId}", api.serve_SessionHistory)
	routeMux.HandleFunc("POST /sessions/{sessionId}/messages", api.serve_SessionMessage)
	routeMux.HandleFunc("GET /documents", api.serve_ListDocuments)
	routeMux.HandleFunc("POST /documents", api.serve_UploadDocument)
	routeMux.HandleFunc("GET /documents/{documentId}/file", api.serve_DocumentFile)
	routeMux.HandleFunc("GET /users", api.serve_ListUsers)
	routeMux.HandleFunc("POST /users", api.serve_CreateUser)
	routeMux.HandleFunc("PATCH /users/{userId}", api.serve_PatchUser)
	routeMux.HandleFunc("GET /audit", api.serve_Audit)
	routeMux.HandleFunc("POST /drafts/check", api.serve_DraftCheck)
	return routeMux
}

func (api *liveAPI) serve_Login(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, err := create_RandomToken()
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	result, err := api.store.Login_User(request.Context(), body.Login, body.Password, token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, loginResponse{
		Token:       result.Token,
		UserID:      result.UserID,
		DisplayName: result.DisplayName,
		AccessLevel: result.AccessLevel,
		Role:        role_ForAccessLevel(result.AccessLevel),
	})
}

func (api *liveAPI) serve_Logout(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	if err := api.store.Delete_AuthToken(request.Context(), token); err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (api *liveAPI) serve_CurrentUser(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, err := api.store.Find_AccountByToken(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, profileResponse{
		UserID:      account.UserID,
		DisplayName: account.DisplayName,
		AccessLevel: account.AccessLevel,
		Role:        role_ForAccessLevel(account.AccessLevel),
	})
}

func (api *liveAPI) serve_CreateSession(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	sessionID, err := api.store.Create_Session(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		SessionID string `json:"sessionId"`
	}{SessionID: sessionID})
}

func (api *liveAPI) serve_ListSessions(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	sessions, err := api.store.List_Sessions(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	lines := make([]sessionLine, 0, len(sessions))
	for _, session := range sessions {
		lines = append(lines, sessionLine{
			SessionID: session.SessionID,
			Title:     session.Title,
			CreatedAt: session.CreatedAt.UTC().Format(timeLayout),
			UpdatedAt: session.UpdatedAt.UTC().Format(timeLayout),
		})
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Sessions []sessionLine `json:"sessions"`
	}{Sessions: lines})
}

func (api *liveAPI) serve_SessionHistory(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	messages, err := api.store.List_SessionMessages(request.Context(), token, request.PathValue("sessionId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	rawMessages := make([]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		rawMessages = append(rawMessages, json.RawMessage(message))
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		SessionID string            `json:"sessionId"`
		Messages  []json.RawMessage `json:"messages"`
	}{SessionID: request.PathValue("sessionId"), Messages: rawMessages})
}

func (api *liveAPI) serve_SessionMessage(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil || strings.TrimSpace(body.Text) == "" {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, templateStep, err := api.store.Load_SessionForMessage(request.Context(), token, request.PathValue("sessionId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	session := &sessionRecord{templateStep: templateStep}
	var reply assistantReply
	var documents []shownDocument
	if templateStep == templateStepAsked {
		reply = build_AssistantReply(session, body.Text)
		documents = []shownDocument{}
	} else if strings.Contains(strings.ToLower(body.Text), texts.PhraseTemplate) {
		session.templateStep = templateStepAsked
		reply = build_TemplateFieldReply()
		documents = []shownDocument{}
	} else {
		historyRaw, historyError := api.store.List_SessionMessages(request.Context(), token, request.PathValue("sessionId"))
		if historyError != nil {
			write_LiveError(responseWriter, historyError)
			return
		}
		embedding, embedError := api.store.Embed_Question(body.Text)
		if embedError != nil {
			write_LiveError(responseWriter, embedError)
			return
		}
		chunks, searchError := api.store.Find_ReplyChunks(request.Context(), embedding, account.AccessLevel)
		if searchError != nil {
			write_LiveError(responseWriter, searchError)
			return
		}
		var replyError error
		reply, documents, replyError = build_LiveAssistantReply(api.chatter, body.Text, read_ChatHistory(historyRaw), chunks)
		if replyError != nil {
			write_LiveError(responseWriter, replyError)
			return
		}
	}
	userBody, err := json.Marshal(userLine{Role: "user", Text: body.Text})
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	stored := reply
	stored.Role = "assistant"
	assistantBody, err := json.Marshal(stored)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	if documents == nil {
		documents = []shownDocument{}
	}
	auditDocuments, err := json.Marshal(documents)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	err = api.store.Save_SessionExchange(request.Context(), request.PathValue("sessionId"), session.templateStep, userBody, assistantBody, auditDocuments, account.UserID, account.DisplayName, body.Text, build_SessionTitle(body.Text))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, reply)
}

func (api *liveAPI) serve_ListDocuments(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	lines, err := api.store.List_VisibleDocuments(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	visible := make([]documentResponse, 0, len(lines))
	for _, line := range lines {
		visible = append(visible, documentResponse{
			DocumentID:  line.DocumentID,
			Title:       line.Title,
			Edition:     line.Edition,
			AccessLevel: line.AccessLevel,
		})
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Documents []documentResponse `json:"documents"`
	}{Documents: visible})
}

func (api *liveAPI) serve_UploadDocument(responseWriter http.ResponseWriter, request *http.Request) {
	fileName, body, contentType, err := read_UploadedFile(responseWriter, request)
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	title := strings.TrimSpace(request.FormValue("title"))
	if title == "" {
		title = strings.TrimSpace(fileName)
	}
	if title == "" {
		title = texts.DefaultDocumentTitle
	}
	edition := strings.TrimSpace(request.FormValue("edition"))
	if edition == "" {
		edition = texts.EditionDefault
	}
	accessLevel := texts.SeedDocumentAccessOpen
	if rawLevel := strings.TrimSpace(request.FormValue("accessLevel")); rawLevel != "" {
		parsed, parseErr := strconv.Atoi(rawLevel)
		if parseErr != nil {
			write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
			return
		}
		accessLevel = parsed
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	cleanName := filepath.Base(fileName)
	if cleanName == "" || cleanName == "." {
		cleanName = "document.bin"
	}
	result, err := api.store.Index_UploadedDocument(request.Context(), token, livestore.UploadInput{
		FileName:    cleanName,
		ContentType: contentType,
		Body:        body,
		Title:       title,
		Edition:     edition,
		AccessLevel: accessLevel,
	})
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, uploadResponse{
		DocumentID: result.DocumentID,
		Status:     texts.StatusAccepted,
		Edition:    result.Edition,
	})
}

func (api *liveAPI) serve_DocumentFile(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	file, err := api.store.Read_DocumentFile(request.Context(), token, request.PathValue("documentId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	responseWriter.Header().Set("Content-Type", file.ContentType)
	responseWriter.Header().Set("Content-Disposition", build_AttachmentHeader(file.FileName))
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(file.Body)
}

func (api *liveAPI) serve_ListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	accounts, err := api.store.List_Accounts(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	lines := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		lines = append(lines, accountResponse{
			UserID:      account.UserID,
			Login:       account.Login,
			DisplayName: account.DisplayName,
			AccessLevel: account.AccessLevel,
			Role:        role_ForAccessLevel(account.AccessLevel),
		})
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Users []accountResponse `json:"users"`
	}{Users: lines})
}

func (api *liveAPI) serve_CreateUser(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		Login       string `json:"login"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
		AccessLevel *int   `json:"accessLevel"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	body.Login = strings.TrimSpace(body.Login)
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.Login == "" || body.Password == "" || body.DisplayName == "" {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	accessLevel := texts.SeedDocumentAccessOpen
	if body.AccessLevel != nil {
		accessLevel = *body.AccessLevel
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, err := api.store.Create_Account(request.Context(), token, body.Login, body.Password, body.DisplayName, accessLevel)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, accountResponse{
		UserID:      account.UserID,
		Login:       account.Login,
		DisplayName: account.DisplayName,
		AccessLevel: account.AccessLevel,
		Role:        role_ForAccessLevel(account.AccessLevel),
	})
}

func (api *liveAPI) serve_PatchUser(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		DisplayName *string `json:"displayName"`
		AccessLevel *int    `json:"accessLevel"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if body.DisplayName == nil && body.AccessLevel == nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, err := api.store.Patch_Account(request.Context(), token, request.PathValue("userId"), body.DisplayName, body.AccessLevel)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, accountResponse{
		UserID:      account.UserID,
		Login:       account.Login,
		DisplayName: account.DisplayName,
		AccessLevel: account.AccessLevel,
		Role:        role_ForAccessLevel(account.AccessLevel),
	})
}

func (api *liveAPI) serve_Audit(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	entries, err := api.store.List_Audit(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	lines := make([]auditEntry, 0, len(entries))
	for _, entry := range entries {
		documents := []shownDocument{}
		if len(entry.Documents) > 0 {
			if err := json.Unmarshal(entry.Documents, &documents); err != nil {
				write_LiveError(responseWriter, err)
				return
			}
		}
		lines = append(lines, auditEntry{
			EntryID:     entry.EntryID,
			UserID:      entry.UserID,
			DisplayName: entry.DisplayName,
			Question:    entry.Question,
			CreatedAt:   entry.CreatedAt.UTC().Format(timeLayout),
			Documents:   documents,
		})
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Entries []auditEntry `json:"entries"`
	}{Entries: lines})
}

func (api *liveAPI) serve_DraftCheck(responseWriter http.ResponseWriter, request *http.Request) {
	if _, _, _, err := read_UploadedFile(responseWriter, request); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	if _, err := api.store.Find_AccountByToken(request.Context(), token); err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Recommendations []struct {
			Text     string   `json:"text"`
			Citation citation `json:"citation"`
		} `json:"recommendations"`
	}{
		Recommendations: []struct {
			Text     string   `json:"text"`
			Citation citation `json:"citation"`
		}{
			{
				Text: texts.DraftRecommendation,
				Citation: citation{
					Document:     texts.SeedInstructionTitle,
					Edition:      texts.EditionDefault,
					Section:      texts.DraftSection,
					Paragraph:    texts.DraftParagraph,
					Appendix:     "",
					Quote:        texts.DraftQuote,
					OriginalLink: build_OriginalFileLink(texts.SeedInstructionID, texts.DraftParagraph),
				},
			},
		},
	})
}

func read_BearerToken(request *http.Request) (string, bool) {
	header := request.Header.Get("Authorization")
	token, found := strings.CutPrefix(header, "Bearer ")
	if !found || token == "" || token != strings.TrimSpace(token) {
		return "", false
	}
	return token, true
}

func write_LiveError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, livestore.ErrInvalidCredentials):
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorInvalidCredentials)
	case errors.Is(err, livestore.ErrUnauthorized):
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
	case errors.Is(err, livestore.ErrAccessDenied):
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
	case errors.Is(err, livestore.ErrNotFound):
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
	case errors.Is(err, livestore.ErrLoginTaken):
		write_ErrorResponse(responseWriter, http.StatusConflict, texts.ErrorLoginTaken)
	case errors.Is(err, livestore.ErrUnsupportedFile):
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
	case errors.Is(err, livestore.ErrEmbedSize):
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorEmbedSize)
	case errors.Is(err, livestore.ErrEmbedUnavailable):
		write_ErrorResponse(responseWriter, http.StatusBadGateway, texts.ErrorEmbedUnavailable)
	default:
		log.Printf("store error: %v", err)
		write_ErrorResponse(responseWriter, http.StatusInternalServerError, texts.ErrorStore)
	}
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
