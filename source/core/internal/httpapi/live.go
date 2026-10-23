package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"docssearch/core/internal/auth"
	"docssearch/core/internal/blank"
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
	routeMux.HandleFunc("GET /auth/config", api.serve_AuthConfig)
	routeMux.HandleFunc("POST /auth/signup", api.serve_Signup)
	routeMux.HandleFunc("POST /auth/ldap", api.serve_LdapLogin)
	routeMux.HandleFunc("GET /auth/admin/details", api.serve_AdminDetails)
	routeMux.HandleFunc("GET /auth/admin/config", api.serve_SignupSettings)
	routeMux.HandleFunc("POST /auth/admin/config", api.serve_SaveSignupSettings)
	routeMux.HandleFunc("GET /auth/admin/config/ldap", api.serve_LdapToggle)
	routeMux.HandleFunc("POST /auth/admin/config/ldap", api.serve_SaveLdapToggle)
	routeMux.HandleFunc("GET /auth/admin/config/ldap/server", api.serve_LdapServer)
	routeMux.HandleFunc("POST /auth/admin/config/ldap/server", api.serve_SaveLdapServer)
	routeMux.HandleFunc("GET /me", api.serve_CurrentUser)
	routeMux.HandleFunc("POST /sessions", api.serve_CreateSession)
	routeMux.HandleFunc("GET /sessions", api.serve_ListSessions)
	routeMux.HandleFunc("GET /sessions/{sessionId}", api.serve_SessionHistory)
	routeMux.HandleFunc("POST /sessions/{sessionId}/messages", api.serve_SessionMessage)
	routeMux.HandleFunc("GET /documents", api.serve_ListDocuments)
	routeMux.HandleFunc("POST /documents", api.serve_UploadDocument)
	routeMux.HandleFunc("GET /documents/{documentId}/file", api.serve_DocumentFile)
	routeMux.HandleFunc("GET /blanks/{blankId}/file", api.serve_BlankFile)
	routeMux.HandleFunc("GET /drafts/{draftId}/file", api.serve_DraftFile)
	routeMux.HandleFunc("DELETE /documents/{documentId}", api.serve_DeleteDocument)
	routeMux.HandleFunc("GET /users", api.serve_ListUsers)
	routeMux.HandleFunc("POST /users", api.serve_CreateUser)
	routeMux.HandleFunc("PATCH /users/{userId}", api.serve_PatchUser)
	routeMux.HandleFunc("GET /audit", api.serve_Audit)
	routeMux.HandleFunc("POST /drafts/check", api.serve_DraftCheck)
	return routeMux
}

func (api *liveAPI) serve_Login(responseWriter http.ResponseWriter, request *http.Request) {
	loginName, password, ok := read_LoginRequest(responseWriter, request)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, err := create_RandomToken()
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	result, err := api.store.Login_User(request.Context(), loginName, password, token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LiveLoginResponse(result))
}

func build_LiveLoginResponse(result livestore.LoginResult) loginResponse {
	return loginResponse{
		Token:       result.Token,
		UserID:      result.UserID,
		Login:       result.Login,
		DisplayName: result.DisplayName,
		AccessLevel: result.AccessLevel,
		Role:        auth.Role_ForAccess(result.AccessLevel, result.Pending),
	}
}

func build_LiveAccountResponse(account livestore.Account) accountResponse {
	return accountResponse{
		UserID:      account.UserID,
		Login:       account.Login,
		DisplayName: account.DisplayName,
		AccessLevel: account.AccessLevel,
		Role:        auth.Role_ForAccess(account.AccessLevel, account.Pending),
		CreatedAt:   account.CreatedAt.UTC().Format(timeLayout),
	}
}

func (api *liveAPI) serve_AuthConfig(responseWriter http.ResponseWriter, request *http.Request) {
	publicConfig, err := api.store.Read_PublicAuthConfig(request.Context())
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, publicAuthConfigResponse{
		Onboarding:   publicConfig.Onboarding,
		EnableSignup: publicConfig.EnableSignup,
		EnableLdap:   publicConfig.EnableLdap,
		LdapLabel:    publicConfig.LdapLabel,
	})
}

func (api *liveAPI) serve_Signup(responseWriter http.ResponseWriter, request *http.Request) {
	body, errorCode := read_SignupRequest(responseWriter, request)
	if errorCode != "" {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, errorCode)
		return
	}
	token, err := create_RandomToken()
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	result, err := api.store.Signup_User(request.Context(), body.Email, body.Password, body.DisplayName, token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LiveLoginResponse(result))
}

func (api *liveAPI) serve_LdapLogin(responseWriter http.ResponseWriter, request *http.Request) {
	body, ok := read_LdapLoginRequest(responseWriter, request)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, err := create_RandomToken()
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	result, err := api.store.Login_LdapUser(request.Context(), body.Login, body.Password, token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LiveLoginResponse(result))
}

func (api *liveAPI) serve_AdminDetails(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	details, err := api.store.Read_AdminDetails(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, adminDetailsResponse{Name: details.Name, Email: details.Email})
}

func (api *liveAPI) serve_SignupSettings(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	settings, err := api.store.Read_SignupSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, settings)
}

func (api *liveAPI) serve_SaveSignupSettings(responseWriter http.ResponseWriter, request *http.Request) {
	var body signupSettingsRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	current, err := api.store.Read_SignupSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	updated, ok := apply_SignupSettingsRequest(current, body)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	saved, err := api.store.Save_SignupSettings(request.Context(), token, updated)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, saved)
}

func (api *liveAPI) serve_LdapToggle(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	settings, err := api.store.Read_LdapSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, ldapToggleResponse{EnableLdap: settings.EnableLdap})
}

func (api *liveAPI) serve_SaveLdapToggle(responseWriter http.ResponseWriter, request *http.Request) {
	var body ldapToggleBody
	if err := read_JSONBody(responseWriter, request, &body); err != nil || body.EnableLdap == nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	settings, err := api.store.Read_LdapSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	settings.EnableLdap = *body.EnableLdap
	saved, err := api.store.Save_LdapSettings(request.Context(), token, settings)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, ldapToggleResponse{EnableLdap: saved.EnableLdap})
}

func (api *liveAPI) serve_LdapServer(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	settings, err := api.store.Read_LdapSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LdapServerResponse(settings))
}

func (api *liveAPI) serve_SaveLdapServer(responseWriter http.ResponseWriter, request *http.Request) {
	var body ldapServerRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	current, err := api.store.Read_LdapSettings(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	updated, ok := apply_LdapServerRequest(current, body)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	saved, err := api.store.Save_LdapSettings(request.Context(), token, updated)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LdapServerResponse(saved))
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
	// Профиль отдаём и ожидающему подтверждения: по роли pending клиент показывает экран ожидания
	account, err := api.store.Find_ProfileByToken(request.Context(), token)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, profileResponse{
		UserID:      account.UserID,
		Login:       account.Login,
		DisplayName: account.DisplayName,
		AccessLevel: account.AccessLevel,
		Role:        auth.Role_ForAccess(account.AccessLevel, account.Pending),
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
	account, _, templateState, err := api.store.Load_SessionForMessage(request.Context(), token, request.PathValue("sessionId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	session := &sessionRecord{blankFill: decode_FillState(templateState)}
	var reply assistantReply
	var documents []shownDocument
	step, handled := blank.Advance_Blank(session.blankFill, body.Text)
	if handled {
		apply_BlankStep(session, step)
		reply = reply_FromBlank(step)
		documents = []shownDocument{}
		if step.Ready && len(step.DraftDocx) > 0 && reply.Template != nil {
			draftID, saveError := api.store.Save_DraftFile(request.Context(), account.UserID, step.BlankID+".docx", step.DraftDocx)
			if saveError != nil {
				write_LiveError(responseWriter, saveError)
				return
			}
			reply.Template.DraftLink = draft_FileLink(draftID)
		}
	} else if strings.Contains(strings.ToLower(body.Text), texts.PhrasePrepare) {
		lines, listError := api.store.List_VisibleDocuments(request.Context(), token, "")
		if listError != nil {
			write_LiveError(responseWriter, listError)
			return
		}
		var prepareError error
		reply, prepareError = reply_ForPreparation(instruction_Source(lines))
		if prepareError != nil {
			write_DraftError(responseWriter, prepareError)
			return
		}
		documents = shown_FromCitations(reply.Citations)
		reply = reply_WithContacts(reply, nil)
		if embedding, embedError := api.store.Embed_Question(body.Text); embedError == nil {
			contactChunk, found, contactError := api.store.Find_ContactChunk(request.Context(), embedding, account.UserID, account.AccessLevel)
			if contactError != nil {
				write_LiveError(responseWriter, contactError)
				return
			}
			if found {
				reply = reply_WithContacts(reply, contacts_ForText(contactChunk.Content, body.Text))
			}
		}
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
		visible, closed, searchError := api.store.Find_ReplyChunks(request.Context(), embedding, account.UserID, account.AccessLevel)
		if searchError != nil {
			write_LiveError(responseWriter, searchError)
			return
		}
		var replyError error
		reply, documents, replyError = build_LiveAssistantReply(api.chatter, body.Text, read_ChatHistory(historyRaw), visible, closed)
		if replyError != nil {
			write_LiveError(responseWriter, replyError)
			return
		}
		if reply.Kind == texts.KindAnswer || reply.Kind == texts.KindFallback {
			contactChunk, found, contactError := api.store.Find_ContactChunk(request.Context(), embedding, account.UserID, account.AccessLevel)
			if contactError != nil {
				write_LiveError(responseWriter, contactError)
				return
			}
			contactList := []replyContact{}
			if found {
				contactList = contacts_ForText(contactChunk.Content, body.Text)
			}
			reply = reply_WithContacts(reply, contactList)
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
	err = api.store.Save_SessionExchange(request.Context(), request.PathValue("sessionId"), session.templateStep, encode_FillState(session.blankFill), userBody, assistantBody, auditDocuments, account.UserID, account.DisplayName, body.Text, build_SessionTitle(body.Text))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, reply)
}

func (api *liveAPI) serve_ListDocuments(responseWriter http.ResponseWriter, request *http.Request) {
	scopeFilter, scopeOK := read_ListScope(request.URL.Query().Get("scope"))
	if !scopeOK {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
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
	lines, err := api.store.List_VisibleDocuments(request.Context(), token, scopeFilter)
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
			Scope:       line.Scope,
			OwnerID:     line.OwnerID,
			OwnerName:   line.OwnerName,
			UploadedAt:  line.UploadedAt.UTC().Format(time.RFC3339),
			CanDelete:   check_CanDeleteDocument(account.UserID, account.AccessLevel, line.Scope, line.OwnerID),
		})
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		CanUploadGlobal   bool               `json:"canUploadGlobal"`
		CanUploadPersonal bool               `json:"canUploadPersonal"`
		Documents         []documentResponse `json:"documents"`
	}{
		CanUploadGlobal:   check_CanUploadGlobal(account.AccessLevel),
		CanUploadPersonal: true,
		Documents:         visible,
	})
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
	scope, scopeOK := read_UploadScope(request.FormValue("scope"), account.AccessLevel)
	if !scopeOK {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if scope == texts.ScopeGlobal && !check_CanUploadGlobal(account.AccessLevel) {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	accessLevel := account.AccessLevel
	ownerID := ""
	if scope == texts.ScopePersonal {
		ownerID = account.UserID
	} else {
		parsed, levelOK := read_AccessLevel(request.FormValue("accessLevel"), texts.SeedDocumentAccessOpen)
		if !levelOK {
			write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
			return
		}
		accessLevel = parsed
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
		Scope:       scope,
		OwnerID:     ownerID,
	})
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, uploadResponse{
		DocumentID: result.DocumentID,
		Status:     texts.StatusAccepted,
		Edition:    result.Edition,
		Scope:      result.Scope,
	})
}

func (api *liveAPI) serve_BlankFile(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	if _, err := api.store.Find_AccountByToken(request.Context(), token); err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	fileName, body, err := blank.Read_ModuleFile(request.PathValue("blankId"))
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	write_FileResponse(responseWriter, fileName, texts.DocxContentType, body)
}

func (api *liveAPI) serve_DraftFile(responseWriter http.ResponseWriter, request *http.Request) {
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
	file, err := api.store.Read_DraftFile(request.Context(), account.UserID, request.PathValue("draftId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_FileResponse(responseWriter, file.FileName, file.ContentType, file.Body)
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

func (api *liveAPI) serve_DeleteDocument(responseWriter http.ResponseWriter, request *http.Request) {
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	err := api.store.Delete_Document(request.Context(), token, request.PathValue("documentId"))
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
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
		lines = append(lines, build_LiveAccountResponse(account))
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Users []accountResponse `json:"users"`
	}{Users: lines})
}

func (api *liveAPI) serve_CreateUser(responseWriter http.ResponseWriter, request *http.Request) {
	created, ok := read_CreateUserRequest(responseWriter, request)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, err := api.store.Create_Account(request.Context(), token, created.login, created.password, created.displayName, created.accessLevel, created.pending)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LiveAccountResponse(account))
}

func (api *liveAPI) serve_PatchUser(responseWriter http.ResponseWriter, request *http.Request) {
	patch, ok := read_PatchUserRequest(responseWriter, request)
	if !ok {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	account, err := api.store.Patch_Account(request.Context(), token, request.PathValue("userId"), patch)
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, build_LiveAccountResponse(account))
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
	fileName, body, _, err := read_UploadedFile(responseWriter, request)
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	token, found := read_BearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	lines, err := api.store.List_VisibleDocuments(request.Context(), token, "")
	if err != nil {
		write_LiveError(responseWriter, err)
		return
	}
	recommendations, err := recommendations_ForDraft(fileName, body, instruction_Source(lines))
	if err != nil {
		write_DraftError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Recommendations []draftRecommendation `json:"recommendations"`
	}{Recommendations: recommendations})
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
	case errors.Is(err, livestore.ErrSignupDisabled):
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorSignupDisabled)
	case errors.Is(err, livestore.ErrAccountPending):
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccountPending)
	case errors.Is(err, livestore.ErrLdapDisabled):
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorLdapDisabled)
	case errors.Is(err, auth.ErrLdapEmailMissing):
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorLdapEmailMissing)
	case errors.Is(err, auth.ErrLdapUnavailable):
		// В тексте ошибки адрес сервера и ответ каталога, пароля там нет
		log.Printf("ldap error: %v", err)
		write_ErrorResponse(responseWriter, http.StatusBadGateway, texts.ErrorLdapUnavailable)
	default:
		log.Printf("store error: %v", err)
		write_ErrorResponse(responseWriter, http.StatusInternalServerError, texts.ErrorStore)
	}
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
