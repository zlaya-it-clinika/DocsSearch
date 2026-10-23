package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"docssearch/core/internal/blank"
	"docssearch/core/internal/chunk"
	"docssearch/core/internal/contacts"
	"docssearch/core/internal/draftcheck"
	"docssearch/core/internal/texts"
)

const (
	maxUploadBytes    = 20 << 20
	adminAccessLevel  = 2
	templateStepIdle  = 0
	templateStepAsked = 1
)

type userRecord struct {
	id          string
	login       string
	password    string
	displayName string
	accessLevel int
}

type documentRecord struct {
	id          string
	title       string
	edition     string
	accessLevel int
	scope       string
	ownerID     string
	uploadedAt  time.Time
	fileName    string
	contentType string
	body        []byte
}

type sessionRecord struct {
	id           string
	ownerID      string
	title        string
	createdAt    time.Time
	updatedAt    time.Time
	messages     []any
	templateStep int
	blankFill    blank.FillState
}

type citation struct {
	Document     string `json:"document"`
	Edition      string `json:"edition"`
	Section      string `json:"section"`
	Paragraph    string `json:"paragraph"`
	Appendix     string `json:"appendix"`
	Quote        string `json:"quote"`
	OriginalLink string `json:"originalLink"`
	DocumentID   string `json:"documentId"`
	Scope        string `json:"scope"`
}

type replyContact struct {
	Department string `json:"department"`
	Channel    string `json:"channel"`
}

type templateState struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Field      string `json:"field,omitempty"`
	FieldLabel string `json:"fieldLabel,omitempty"`
	DraftText  string `json:"draftText,omitempty"`
	FileLink   string `json:"fileLink,omitempty"`
	DraftLink  string `json:"draftLink,omitempty"`
}

type draftFile struct {
	ownerID     string
	fileName    string
	contentType string
	body        []byte
}

type assistantReply struct {
	Role             string          `json:"role,omitempty"`
	Kind             string          `json:"kind"`
	Text             string          `json:"text"`
	RelevancePercent int             `json:"relevancePercent"`
	Citations        []citation      `json:"citations"`
	Contacts         *[]replyContact `json:"contacts,omitempty"`
	Template         *templateState  `json:"template,omitempty"`
}

type userLine struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type loginResponse struct {
	Token       string `json:"token"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AccessLevel int    `json:"accessLevel"`
	Role        string `json:"role"`
}

type profileResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AccessLevel int    `json:"accessLevel"`
	Role        string `json:"role"`
}

type accountResponse struct {
	UserID      string `json:"userId"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	AccessLevel int    `json:"accessLevel"`
	Role        string `json:"role"`
}

type sessionLine struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type documentResponse struct {
	DocumentID  string `json:"documentId"`
	Title       string `json:"title"`
	Edition     string `json:"edition"`
	AccessLevel int    `json:"accessLevel"`
	Scope       string `json:"scope"`
	OwnerID     string `json:"ownerId"`
	OwnerName   string `json:"ownerName"`
	UploadedAt  string `json:"uploadedAt"`
	CanDelete   bool   `json:"canDelete"`
}

type uploadResponse struct {
	DocumentID string `json:"documentId"`
	Status     string `json:"status"`
	Edition    string `json:"edition"`
	Scope      string `json:"scope"`
}

type shownDocument struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	Paragraph  string `json:"paragraph"`
	Scope      string `json:"scope"`
}

type auditEntry struct {
	EntryID     string          `json:"entryId"`
	UserID      string          `json:"userId"`
	DisplayName string          `json:"displayName"`
	Question    string          `json:"question"`
	CreatedAt   string          `json:"createdAt"`
	Documents   []shownDocument `json:"documents"`
}

type stub struct {
	mu            sync.Mutex
	users         []*userRecord
	usersByID     map[string]*userRecord
	usersByLogin  map[string]*userRecord
	tokens        map[string]string
	documents     []*documentRecord
	documentsByID map[string]*documentRecord
	sessions      map[string]*sessionRecord
	draftsByID    map[string]*draftFile
	audit         []auditEntry
	nextUser      int
	nextDocument  int
	nextSession   int
	nextDraft     int
	nextAudit     int
}

func create_StubStore() *stub {
	stubStore := &stub{
		usersByID:     map[string]*userRecord{},
		usersByLogin:  map[string]*userRecord{},
		tokens:        map[string]string{},
		documentsByID: map[string]*documentRecord{},
		sessions:      map[string]*sessionRecord{},
		draftsByID:    map[string]*draftFile{},
		nextUser:      2,
		nextDocument:  3,
		nextSession:   1,
		nextDraft:     1,
		nextAudit:     1,
	}
	stubStore.addUser(texts.SeedAdminID, texts.SeedAdminLogin, texts.SeedAdminPassword, texts.SeedAdminDisplayName, texts.SeedAdminAccessLevel)
	stubStore.addUser(texts.SeedEmployeeID, texts.SeedEmployeeLogin, texts.SeedEmployeePassword, texts.SeedEmployeeDisplayName, texts.SeedEmployeeAccessLevel)
	stubStore.addDocument(texts.SeedInstructionID, texts.SeedInstructionTitle, texts.EditionDefault, texts.SeedDocumentAccessOpen, texts.SeedInstructionFileName, texts.PlainTextContentType, []byte(texts.SeedInstructionBody))
	stubStore.addDocument(texts.SeedRestrictedID, texts.SeedRestrictedTitle, texts.EditionDefault, texts.SeedDocumentAccessClosed, texts.SeedRestrictedFileName, texts.PlainTextContentType, []byte(texts.SeedRestrictedBody))
	return stubStore
}

func (stubStore *stub) addUser(id, login, password, displayName string, accessLevel int) {
	record := &userRecord{
		id:          id,
		login:       login,
		password:    password,
		displayName: displayName,
		accessLevel: accessLevel,
	}
	stubStore.users = append(stubStore.users, record)
	stubStore.usersByID[id] = record
	stubStore.usersByLogin[login] = record
}

func (stubStore *stub) addDocument(id, title, edition string, accessLevel int, fileName, contentType string, body []byte) {
	record := &documentRecord{
		id:          id,
		title:       title,
		edition:     edition,
		accessLevel: accessLevel,
		scope:       texts.ScopeGlobal,
		uploadedAt:  time.Now().UTC(),
		fileName:    fileName,
		contentType: contentType,
		body:        body,
	}
	stubStore.documents = append(stubStore.documents, record)
	stubStore.documentsByID[id] = record
}

func (stubStore *stub) register_StubRoutes() *http.ServeMux {
	routeMux := http.NewServeMux()
	routeMux.HandleFunc("POST /auth/login", stubStore.serve_Login)
	routeMux.HandleFunc("POST /auth/logout", stubStore.serve_Logout)
	routeMux.HandleFunc("GET /me", stubStore.serve_CurrentUser)
	routeMux.HandleFunc("POST /sessions", stubStore.serve_CreateSession)
	routeMux.HandleFunc("GET /sessions", stubStore.serve_ListSessions)
	routeMux.HandleFunc("GET /sessions/{sessionId}", stubStore.serve_SessionHistory)
	routeMux.HandleFunc("POST /sessions/{sessionId}/messages", stubStore.serve_SessionMessage)
	routeMux.HandleFunc("GET /documents", stubStore.serve_ListDocuments)
	routeMux.HandleFunc("POST /documents", stubStore.serve_UploadDocument)
	routeMux.HandleFunc("GET /documents/{documentId}/file", stubStore.serve_DocumentFile)
	routeMux.HandleFunc("DELETE /documents/{documentId}", stubStore.serve_DeleteDocument)
	routeMux.HandleFunc("GET /blanks/{blankId}/file", stubStore.serve_BlankFile)
	routeMux.HandleFunc("GET /drafts/{draftId}/file", stubStore.serve_DraftFile)
	routeMux.HandleFunc("GET /users", stubStore.serve_ListUsers)
	routeMux.HandleFunc("POST /users", stubStore.serve_CreateUser)
	routeMux.HandleFunc("PATCH /users/{userId}", stubStore.serve_PatchUser)
	routeMux.HandleFunc("GET /audit", stubStore.serve_Audit)
	routeMux.HandleFunc("POST /drafts/check", stubStore.serve_DraftCheck)
	return routeMux
}

func (stubStore *stub) serve_Login(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record := stubStore.usersByLogin[body.Login]
	if record == nil || !match_Password(record.password, body.Password) {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorInvalidCredentials)
		return
	}
	token, err := create_RandomToken()
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusInternalServerError, texts.ErrorInvalidBody)
		return
	}
	stubStore.tokens[token] = record.id
	write_JSONResponse(responseWriter, http.StatusOK, loginResponse{
		Token:       token,
		UserID:      record.id,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
		Role:        role_ForAccessLevel(record.accessLevel),
	})
}

func (stubStore *stub) serve_Logout(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	token, found := read_BearerToken(request)
	if !found || stubStore.tokens[token] == "" {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	delete(stubStore.tokens, token)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (stubStore *stub) serve_CurrentUser(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, profileResponse{
		UserID:      record.id,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
		Role:        role_ForAccessLevel(record.accessLevel),
	})
}

func (stubStore *stub) serve_CreateSession(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	sessionID := "session-" + strconv.Itoa(stubStore.nextSession)
	stubStore.nextSession++
	now := time.Now().UTC()
	stubStore.sessions[sessionID] = &sessionRecord{
		id:        sessionID,
		ownerID:   record.id,
		createdAt: now,
		updatedAt: now,
		messages:  []any{},
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		SessionID string `json:"sessionId"`
	}{SessionID: sessionID})
}

func (stubStore *stub) serve_ListSessions(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	lines := []sessionLine{}
	for _, session := range stubStore.sessions {
		if session.ownerID != record.id {
			continue
		}
		lines = append(lines, session.line())
	}
	sort.Slice(lines, func(left, right int) bool {
		if lines[left].UpdatedAt == lines[right].UpdatedAt {
			return lines[left].SessionID > lines[right].SessionID
		}
		return lines[left].UpdatedAt > lines[right].UpdatedAt
	})
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Sessions []sessionLine `json:"sessions"`
	}{Sessions: lines})
}

func (stubStore *stub) serve_SessionHistory(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	session, found := stubStore.find_OwnedSession(responseWriter, request)
	if !found {
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		SessionID string `json:"sessionId"`
		Messages  []any  `json:"messages"`
	}{SessionID: session.id, Messages: session.messages})
}

func (stubStore *stub) serve_SessionMessage(responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	session, found := stubStore.find_OwnedSession(responseWriter, request)
	if !found {
		return
	}
	record := stubStore.usersByID[session.ownerID]
	reply, step, replyError := build_AssistantReply(session, body.Text)
	if replyError != nil {
		write_DraftError(responseWriter, replyError)
		return
	}
	if step.Ready && len(step.DraftDocx) > 0 && reply.Template != nil {
		draftID := stubStore.save_DraftFile(record.id, step.BlankID+".docx", step.DraftDocx)
		reply.Template.DraftLink = draft_FileLink(draftID)
	}
	if reply.Kind == texts.KindAnswer || reply.Kind == texts.KindFallback {
		reply = reply_WithContacts(reply, stubStore.lookup_Contacts(record, body.Text))
	}
	if session.title == "" {
		session.title = build_SessionTitle(body.Text)
	}
	session.updatedAt = time.Now().UTC()
	session.messages = append(session.messages, userLine{Role: "user", Text: body.Text})
	stored := reply
	stored.Role = "assistant"
	session.messages = append(session.messages, stored)
	stubStore.append_AuditEntry(record, body.Text, reply.Kind)
	write_JSONResponse(responseWriter, http.StatusOK, reply)
}

func (stubStore *stub) serve_ListDocuments(responseWriter http.ResponseWriter, request *http.Request) {
	scopeFilter, scopeOK := read_ListScope(request.URL.Query().Get("scope"))
	if !scopeOK {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	if scopeFilter == texts.ScopeAll && record.accessLevel < adminAccessLevel {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	visible := make([]documentResponse, 0)
	for _, document := range stubStore.documents {
		if !document_MatchesList(scopeFilter, document.scope, document.ownerID, record.id, document.accessLevel, record.accessLevel) {
			continue
		}
		visible = append(visible, document.line(record, stubStore.ownerName(document)))
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		CanUploadGlobal   bool               `json:"canUploadGlobal"`
		CanUploadPersonal bool               `json:"canUploadPersonal"`
		Documents         []documentResponse `json:"documents"`
	}{
		CanUploadGlobal:   check_CanUploadGlobal(record.accessLevel),
		CanUploadPersonal: true,
		Documents:         visible,
	})
}

func (stubStore *stub) serve_UploadDocument(responseWriter http.ResponseWriter, request *http.Request) {
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
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	scope, scopeOK := read_UploadScope(request.FormValue("scope"), record.accessLevel)
	if !scopeOK {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if scope == texts.ScopeGlobal && !check_CanUploadGlobal(record.accessLevel) {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	accessLevel := record.accessLevel
	if scope == texts.ScopeGlobal {
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
	ownerID := ""
	if scope == texts.ScopePersonal {
		ownerID = record.id
	}
	now := time.Now().UTC()
	for _, document := range stubStore.documents {
		if !same_DocumentTitle(document, title, scope, ownerID) {
			continue
		}
		document.edition = edition
		document.accessLevel = accessLevel
		document.fileName = cleanName
		document.contentType = contentType
		document.body = body
		document.uploadedAt = now
		write_JSONResponse(responseWriter, http.StatusOK, uploadResponse{
			DocumentID: document.id,
			Status:     texts.StatusAccepted,
			Edition:    document.edition,
			Scope:      document.scope,
		})
		return
	}
	documentID := "doc-" + strconv.Itoa(stubStore.nextDocument)
	stubStore.nextDocument++
	created := &documentRecord{
		id:          documentID,
		title:       title,
		edition:     edition,
		accessLevel: accessLevel,
		scope:       scope,
		ownerID:     ownerID,
		uploadedAt:  now,
		fileName:    cleanName,
		contentType: contentType,
		body:        body,
	}
	stubStore.documents = append(stubStore.documents, created)
	stubStore.documentsByID[documentID] = created
	write_JSONResponse(responseWriter, http.StatusOK, uploadResponse{
		DocumentID: documentID,
		Status:     texts.StatusAccepted,
		Edition:    edition,
		Scope:      scope,
	})
}

func (stubStore *stub) serve_DocumentFile(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	document := stubStore.documentsByID[request.PathValue("documentId")]
	if document == nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if status := document_ReadStatus(record.id, record.accessLevel, document.scope, document.ownerID, document.accessLevel); status != http.StatusOK {
		if status == http.StatusNotFound {
			write_ErrorResponse(responseWriter, status, texts.ErrorNotFound)
			return
		}
		write_ErrorResponse(responseWriter, status, texts.ErrorAccessDenied)
		return
	}
	responseWriter.Header().Set("Content-Type", document.contentType)
	responseWriter.Header().Set("Content-Disposition", build_AttachmentHeader(document.fileName))
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(document.body)
}

func (stubStore *stub) serve_DeleteDocument(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	documentID := request.PathValue("documentId")
	document := stubStore.documentsByID[documentID]
	if document == nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if status := document_ReadStatus(record.id, record.accessLevel, document.scope, document.ownerID, document.accessLevel); status != http.StatusOK {
		if status == http.StatusNotFound {
			write_ErrorResponse(responseWriter, status, texts.ErrorNotFound)
			return
		}
		write_ErrorResponse(responseWriter, status, texts.ErrorAccessDenied)
		return
	}
	if !check_CanDeleteDocument(record.id, record.accessLevel, document.scope, document.ownerID) {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	delete(stubStore.documentsByID, documentID)
	remaining := stubStore.documents[:0]
	for _, item := range stubStore.documents {
		if item.id != documentID {
			remaining = append(remaining, item)
		}
	}
	stubStore.documents = remaining
	responseWriter.WriteHeader(http.StatusNoContent)
}

func same_DocumentTitle(document *documentRecord, title, scope, ownerID string) bool {
	if document.title != title || document.scope != scope {
		return false
	}
	if scope == texts.ScopePersonal {
		return document.ownerID == ownerID
	}
	return true
}

func (stubStore *stub) ownerName(document *documentRecord) string {
	if document.scope != texts.ScopePersonal {
		return ""
	}
	owner := stubStore.usersByID[document.ownerID]
	if owner == nil {
		return ""
	}
	return owner.displayName
}

func (stubStore *stub) serve_ListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.require_AdminUser(responseWriter, request); !found {
		return
	}
	accounts := make([]accountResponse, 0, len(stubStore.users))
	for _, record := range stubStore.users {
		accounts = append(accounts, record.account())
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Users []accountResponse `json:"users"`
	}{Users: accounts})
}

func (stubStore *stub) serve_CreateUser(responseWriter http.ResponseWriter, request *http.Request) {
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
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.require_AdminUser(responseWriter, request); !found {
		return
	}
	if stubStore.usersByLogin[body.Login] != nil {
		write_ErrorResponse(responseWriter, http.StatusConflict, texts.ErrorLoginTaken)
		return
	}
	userID := "user-" + strconv.Itoa(stubStore.nextUser)
	stubStore.nextUser++
	stubStore.addUser(userID, body.Login, body.Password, body.DisplayName, accessLevel)
	write_JSONResponse(responseWriter, http.StatusOK, stubStore.usersByID[userID].account())
}

func (stubStore *stub) serve_PatchUser(responseWriter http.ResponseWriter, request *http.Request) {
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
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.require_AdminUser(responseWriter, request); !found {
		return
	}
	record := stubStore.usersByID[request.PathValue("userId")]
	if record == nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if body.DisplayName != nil {
		record.displayName = strings.TrimSpace(*body.DisplayName)
	}
	if body.AccessLevel != nil {
		record.accessLevel = *body.AccessLevel
	}
	write_JSONResponse(responseWriter, http.StatusOK, record.account())
}

func (stubStore *stub) serve_Audit(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.require_AdminUser(responseWriter, request); !found {
		return
	}
	entries := stubStore.audit
	if entries == nil {
		entries = []auditEntry{}
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Entries []auditEntry `json:"entries"`
	}{Entries: entries})
}

func (stubStore *stub) serve_DraftCheck(responseWriter http.ResponseWriter, request *http.Request) {
	fileName, body, _, err := read_UploadedFile(responseWriter, request)
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.find_UserByBearerToken(request); !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	recommendations, err := recommendations_ForDraft(fileName, body, draftcheck.Source{
		DocumentID: texts.SeedInstructionID,
		Title:      texts.SeedInstructionTitle,
		Edition:    texts.EditionDefault,
		Scope:      texts.ScopeGlobal,
	})
	if err != nil {
		write_DraftError(responseWriter, err)
		return
	}
	write_JSONResponse(responseWriter, http.StatusOK, struct {
		Recommendations []draftRecommendation `json:"recommendations"`
	}{Recommendations: recommendations})
}

func recommendations_ForDraft(fileName string, body []byte, source draftcheck.Source) ([]draftRecommendation, error) {
	plain, err := chunk.Extract_DocumentText(fileName, body)
	if err != nil {
		return nil, err
	}
	items, err := draftcheck.Check_Draft(plain, source)
	if err != nil {
		return nil, err
	}
	recommendations := make([]draftRecommendation, 0, len(items))
	for _, item := range items {
		link := ""
		if item.DocumentID != "" && item.Paragraph != "" {
			link = build_OriginalFileLink(item.DocumentID, item.Paragraph)
		}
		recommendations = append(recommendations, draftRecommendation{
			Text: item.Text,
			Citation: citation{
				Document:     item.Title,
				Edition:      item.Edition,
				Section:      item.Section,
				Paragraph:    item.Paragraph,
				Appendix:     item.Appendix,
				Quote:        item.Quote,
				OriginalLink: link,
				DocumentID:   item.DocumentID,
				Scope:        item.Scope,
			},
		})
	}
	return recommendations, nil
}

func write_DraftError(responseWriter http.ResponseWriter, err error) {
	if errors.Is(err, chunk.ErrUnsupportedFile) {
		write_ErrorResponse(responseWriter, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	write_ErrorResponse(responseWriter, http.StatusInternalServerError, texts.ErrorStore)
}

type draftRecommendation struct {
	Text     string   `json:"text"`
	Citation citation `json:"citation"`
}

func (stubStore *stub) find_OwnedSession(responseWriter http.ResponseWriter, request *http.Request) (*sessionRecord, bool) {
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return nil, false
	}
	session := stubStore.sessions[request.PathValue("sessionId")]
	if session == nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return nil, false
	}
	if session.ownerID != record.id {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return nil, false
	}
	return session, true
}

func (stubStore *stub) require_AdminUser(responseWriter http.ResponseWriter, request *http.Request) (*userRecord, bool) {
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return nil, false
	}
	if record.accessLevel < adminAccessLevel {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return nil, false
	}
	return record, true
}

func (stubStore *stub) find_UserByBearerToken(request *http.Request) (*userRecord, bool) {
	header := request.Header.Get("Authorization")
	token, found := strings.CutPrefix(header, "Bearer ")
	if !found || token == "" || token != strings.TrimSpace(token) {
		return nil, false
	}
	userID := stubStore.tokens[token]
	record := stubStore.usersByID[userID]
	if record == nil {
		return nil, false
	}
	return record, true
}

func (stubStore *stub) append_AuditEntry(record *userRecord, question, kind string) {
	documents := []shownDocument{}
	if kind == texts.KindAnswer {
		documents = []shownDocument{{
			DocumentID: texts.SeedInstructionID,
			Title:      texts.SeedInstructionTitle,
			Paragraph:  texts.AnswerParagraph,
			Scope:      texts.ScopeGlobal,
		}}
	}
	stubStore.audit = append(stubStore.audit, auditEntry{
		EntryID:     "audit-" + strconv.Itoa(stubStore.nextAudit),
		UserID:      record.id,
		DisplayName: record.displayName,
		Question:    question,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Documents:   documents,
	})
	stubStore.nextAudit++
}

func (record *userRecord) account() accountResponse {
	return accountResponse{
		UserID:      record.id,
		Login:       record.login,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
		Role:        role_ForAccessLevel(record.accessLevel),
	}
}

func (session *sessionRecord) line() sessionLine {
	return sessionLine{
		SessionID: session.id,
		Title:     session.title,
		CreatedAt: session.createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: session.updatedAt.UTC().Format(time.RFC3339),
	}
}

func role_ForAccessLevel(accessLevel int) string {
	if accessLevel >= adminAccessLevel {
		return texts.RoleAdmin
	}
	return texts.RoleUser
}

// Заголовок чата берётся из первого вопроса и дальше не меняется.
func build_SessionTitle(text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	runes := []rune(collapsed)
	const titleRuneLimit = 60
	if len(runes) <= titleRuneLimit {
		return collapsed
	}
	return strings.TrimSpace(string(runes[:titleRuneLimit-3])) + "..."
}

func (document *documentRecord) line(viewer *userRecord, ownerName string) documentResponse {
	return documentResponse{
		DocumentID:  document.id,
		Title:       document.title,
		Edition:     document.edition,
		AccessLevel: document.accessLevel,
		Scope:       document.scope,
		OwnerID:     document.ownerID,
		OwnerName:   ownerName,
		UploadedAt:  document.uploadedAt.UTC().Format(time.RFC3339),
		CanDelete:   check_CanDeleteDocument(viewer.id, viewer.accessLevel, document.scope, document.ownerID),
	}
}

func (stubStore *stub) lookup_Contacts(record *userRecord, question string) []replyContact {
	if record == nil {
		return []replyContact{}
	}
	entries := []contacts.Entry{}
	for _, document := range stubStore.documents {
		if document == nil || !contacts.Title_IsDirectory(document.title) {
			continue
		}
		if document_ReadStatus(record.id, record.accessLevel, document.scope, document.ownerID, document.accessLevel) != http.StatusOK {
			continue
		}
		plain, extractError := chunk.Extract_DocumentText(document.fileName, document.body)
		if extractError != nil {
			continue
		}
		entries = append(entries, contacts.Parse_Entries(plain)...)
	}
	return contacts_FromEntries(entries, question)
}

// Пока бланк заполняется, реплика - ответ на поле, а не новая фраза заглушки.
func build_AssistantReply(session *sessionRecord, text string) (assistantReply, blank.Step, error) {
	step, handled := blank.Advance_Blank(session.blankFill, text)
	if handled {
		apply_BlankStep(session, step)
		return reply_FromBlank(step), step, nil
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, texts.PhraseAccessDenied):
		return build_PlainReply(texts.KindAccessDenied, texts.AccessDeniedText), blank.Step{}, nil
	case strings.Contains(lower, texts.PhraseNotFound):
		return build_PlainReply(texts.KindFallback, texts.FallbackText), blank.Step{}, nil
	case strings.Contains(lower, texts.PhraseClarify):
		return build_PlainReply(texts.KindClarify, texts.ClarifyText), blank.Step{}, nil
	case strings.Contains(lower, texts.PhrasePrepare):
		reply, err := reply_ForPreparation(draftcheck.Source{
			DocumentID: texts.SeedInstructionID,
			Title:      texts.SeedInstructionTitle,
			Edition:    texts.EditionDefault,
			Scope:      texts.ScopeGlobal,
		})
		return reply, blank.Step{}, err
	default:
		return build_AnswerReply(), blank.Step{}, nil
	}
}

func apply_BlankStep(session *sessionRecord, step blank.Step) {
	session.blankFill = step.State
	session.templateStep = templateStepIdle
	if step.State.BlankID != "" {
		session.templateStep = templateStepAsked
	}
}

func reply_FromBlank(step blank.Step) assistantReply {
	if step.Ready {
		reply := build_PlainReply(texts.KindTemplateReady, texts.TemplateReadyText)
		reply.Template = &templateState{
			ID:        step.BlankID,
			Title:     step.Title,
			DraftText: step.DraftText,
			FileLink:  blank_FileLink(step.BlankID),
		}
		return reply
	}
	reply := build_PlainReply(texts.KindTemplateField, step.Question)
	reply.Template = &templateState{
		ID:         step.BlankID,
		Title:      step.Title,
		Field:      step.Field,
		FieldLabel: step.FieldLabel,
		FileLink:   blank_FileLink(step.BlankID),
	}
	return reply
}

func (stubStore *stub) save_DraftFile(ownerID, fileName string, body []byte) string {
	draftID := "draft-" + strconv.Itoa(stubStore.nextDraft)
	stubStore.nextDraft++
	stubStore.draftsByID[draftID] = &draftFile{
		ownerID:     ownerID,
		fileName:    fileName,
		contentType: texts.DocxContentType,
		body:        body,
	}
	return draftID
}

func (stubStore *stub) serve_BlankFile(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	if _, found := stubStore.find_UserByBearerToken(request); !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	fileName, body, err := blank.Read_ModuleFile(request.PathValue("blankId"))
	if err != nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	write_FileResponse(responseWriter, fileName, texts.DocxContentType, body)
}

func (stubStore *stub) serve_DraftFile(responseWriter http.ResponseWriter, request *http.Request) {
	stubStore.mu.Lock()
	defer stubStore.mu.Unlock()
	record, found := stubStore.find_UserByBearerToken(request)
	if !found {
		write_ErrorResponse(responseWriter, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	file := stubStore.draftsByID[request.PathValue("draftId")]
	if file == nil {
		write_ErrorResponse(responseWriter, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if file.ownerID != record.id {
		write_ErrorResponse(responseWriter, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	write_FileResponse(responseWriter, file.fileName, file.contentType, file.body)
}

func encode_FillState(state blank.FillState) []byte {
	if state.BlankID == "" && len(state.Answers) == 0 && len(state.Skipped) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func decode_FillState(raw []byte) blank.FillState {
	var state blank.FillState
	if len(raw) == 0 {
		return state
	}
	_ = json.Unmarshal(raw, &state)
	return state
}

func build_PlainReply(kind, text string) assistantReply {
	return assistantReply{
		Kind:             kind,
		Text:             text,
		RelevancePercent: 0,
		Citations:        []citation{},
	}
}

func build_AnswerReply() assistantReply {
	return assistantReply{
		Kind:             texts.KindAnswer,
		Text:             texts.AnswerText,
		RelevancePercent: texts.AnswerRelevancePercent,
		Citations: []citation{{
			Document:     texts.SeedInstructionTitle,
			Edition:      texts.EditionDefault,
			Section:      texts.AnswerSection,
			Paragraph:    texts.AnswerParagraph,
			Appendix:     "",
			Quote:        texts.AnswerQuote,
			OriginalLink: build_OriginalFileLink(texts.SeedInstructionID, texts.AnswerParagraph),
			DocumentID:   texts.SeedInstructionID,
			Scope:        texts.ScopeGlobal,
		}},
	}
}

func build_OriginalFileLink(documentID, paragraph string) string {
	return "/documents/" + documentID + "/file#p" + paragraph
}

func match_Password(stored, given string) bool {
	if len(stored) != len(given) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(given)) == 1
}

func create_RandomToken() (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes), nil
}

func read_UploadedFile(responseWriter http.ResponseWriter, request *http.Request) (string, []byte, string, error) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxUploadBytes)
	if err := request.ParseMultipartForm(maxUploadBytes); err != nil {
		return "", nil, "", err
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		return "", nil, "", err
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		return "", nil, "", err
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	return header.Filename, body, contentType, nil
}

func build_AttachmentHeader(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(symbol rune) rune {
		if symbol == '"' || symbol == '\n' || symbol == '\r' {
			return -1
		}
		return symbol
	}, name)
	if name == "" || name == "." {
		name = "document"
	}
	return `attachment; filename="` + name + `"`
}
