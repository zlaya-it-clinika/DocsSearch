package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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
	fileName    string
	contentType string
	body        []byte
}

type sessionRecord struct {
	id           string
	ownerID      string
	messages     []any
	templateStep int
}

type citation struct {
	Document     string `json:"document"`
	Edition      string `json:"edition"`
	Section      string `json:"section"`
	Paragraph    string `json:"paragraph"`
	Appendix     string `json:"appendix"`
	Quote        string `json:"quote"`
	OriginalLink string `json:"originalLink"`
}

type templateState struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Field      string `json:"field,omitempty"`
	FieldLabel string `json:"fieldLabel,omitempty"`
	DraftText  string `json:"draftText,omitempty"`
}

type assistantReply struct {
	Role             string         `json:"role,omitempty"`
	Kind             string         `json:"kind"`
	Text             string         `json:"text"`
	RelevancePercent int            `json:"relevancePercent"`
	Citations        []citation     `json:"citations"`
	Template         *templateState `json:"template,omitempty"`
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
}

type profileResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AccessLevel int    `json:"accessLevel"`
}

type accountResponse struct {
	UserID      string `json:"userId"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	AccessLevel int    `json:"accessLevel"`
}

type documentResponse struct {
	DocumentID  string `json:"documentId"`
	Title       string `json:"title"`
	Edition     string `json:"edition"`
	AccessLevel int    `json:"accessLevel"`
}

type uploadResponse struct {
	DocumentID string `json:"documentId"`
	Status     string `json:"status"`
	Edition    string `json:"edition"`
}

type shownDocument struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	Paragraph  string `json:"paragraph"`
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
	audit         []auditEntry
	nextUser      int
	nextDocument  int
	nextSession   int
	nextAudit     int
}

func newStub() *stub {
	store := &stub{
		usersByID:     map[string]*userRecord{},
		usersByLogin:  map[string]*userRecord{},
		tokens:        map[string]string{},
		documentsByID: map[string]*documentRecord{},
		sessions:      map[string]*sessionRecord{},
		nextUser:      2,
		nextDocument:  3,
		nextSession:   1,
		nextAudit:     1,
	}
	store.addUser(texts.SeedAdminID, texts.SeedAdminLogin, texts.SeedAdminPassword, texts.SeedAdminDisplayName, texts.SeedAdminAccessLevel)
	store.addUser(texts.SeedEmployeeID, texts.SeedEmployeeLogin, texts.SeedEmployeePassword, texts.SeedEmployeeDisplayName, texts.SeedEmployeeAccessLevel)
	store.addDocument(texts.SeedInstructionID, texts.SeedInstructionTitle, texts.EditionDefault, texts.SeedDocumentAccessOpen, texts.SeedInstructionFileName, texts.PlainTextContentType, []byte(texts.SeedInstructionBody))
	store.addDocument(texts.SeedRestrictedID, texts.SeedRestrictedTitle, texts.EditionDefault, texts.SeedDocumentAccessClosed, texts.SeedRestrictedFileName, texts.PlainTextContentType, []byte(texts.SeedRestrictedBody))
	return store
}

func (store *stub) addUser(id, login, password, displayName string, accessLevel int) {
	record := &userRecord{
		id:          id,
		login:       login,
		password:    password,
		displayName: displayName,
		accessLevel: accessLevel,
	}
	store.users = append(store.users, record)
	store.usersByID[id] = record
	store.usersByLogin[login] = record
}

func (store *stub) addDocument(id, title, edition string, accessLevel int, fileName, contentType string, body []byte) {
	record := &documentRecord{
		id:          id,
		title:       title,
		edition:     edition,
		accessLevel: accessLevel,
		fileName:    fileName,
		contentType: contentType,
		body:        body,
	}
	store.documents = append(store.documents, record)
	store.documentsByID[id] = record
}

func (store *stub) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", store.serveLogin)
	mux.HandleFunc("GET /me", store.serveMe)
	mux.HandleFunc("POST /sessions", store.serveCreateSession)
	mux.HandleFunc("GET /sessions/{sessionId}", store.serveSessionHistory)
	mux.HandleFunc("POST /sessions/{sessionId}/messages", store.serveSessionMessage)
	mux.HandleFunc("GET /documents", store.serveListDocuments)
	mux.HandleFunc("POST /documents", store.serveUploadDocument)
	mux.HandleFunc("GET /documents/{documentId}/file", store.serveDocumentFile)
	mux.HandleFunc("GET /users", store.serveListUsers)
	mux.HandleFunc("POST /users", store.serveCreateUser)
	mux.HandleFunc("PATCH /users/{userId}", store.servePatchUser)
	mux.HandleFunc("GET /audit", store.serveAudit)
	mux.HandleFunc("POST /drafts/check", store.serveDraftCheck)
	return mux
}

func (store *stub) serveLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.usersByLogin[body.Login]
	if record == nil || !passwordMatches(record.password, body.Password) {
		writeError(w, http.StatusUnauthorized, texts.ErrorInvalidCredentials)
		return
	}
	token, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, texts.ErrorInvalidBody)
		return
	}
	store.tokens[token] = record.id
	writeJSON(w, http.StatusOK, loginResponse{
		Token:       token,
		UserID:      record.id,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
	})
}

func (store *stub) serveMe(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{
		UserID:      record.id,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
	})
}

func (store *stub) serveCreateSession(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	sessionID := "session-" + strconv.Itoa(store.nextSession)
	store.nextSession++
	store.sessions[sessionID] = &sessionRecord{
		id:       sessionID,
		ownerID:  record.id,
		messages: []any{},
	}
	writeJSON(w, http.StatusOK, struct {
		SessionID string `json:"sessionId"`
	}{SessionID: sessionID})
}

func (store *stub) serveSessionHistory(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.ownedSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		SessionID string `json:"sessionId"`
		Messages  []any  `json:"messages"`
	}{SessionID: session.id, Messages: session.messages})
}

func (store *stub) serveSessionMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.ownedSession(w, r)
	if !ok {
		return
	}
	record := store.usersByID[session.ownerID]
	reply := replyFor(session, body.Text)
	session.messages = append(session.messages, userLine{Role: "user", Text: body.Text})
	stored := reply
	stored.Role = "assistant"
	session.messages = append(session.messages, stored)
	store.appendAudit(record, body.Text, reply.Kind)
	writeJSON(w, http.StatusOK, reply)
}

func (store *stub) serveListDocuments(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	visible := make([]documentResponse, 0)
	for _, document := range store.documents {
		if record.accessLevel < document.accessLevel {
			continue
		}
		visible = append(visible, document.public())
	}
	writeJSON(w, http.StatusOK, struct {
		Documents []documentResponse `json:"documents"`
	}{Documents: visible})
}

func (store *stub) serveUploadDocument(w http.ResponseWriter, r *http.Request) {
	fileName, body, contentType, err := readUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSpace(fileName)
	}
	if title == "" {
		title = texts.DefaultDocumentTitle
	}
	edition := strings.TrimSpace(r.FormValue("edition"))
	if edition == "" {
		edition = texts.EditionDefault
	}
	accessLevel := texts.SeedDocumentAccessOpen
	if rawLevel := strings.TrimSpace(r.FormValue("accessLevel")); rawLevel != "" {
		parsed, parseErr := strconv.Atoi(rawLevel)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
			return
		}
		accessLevel = parsed
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.userFromRequest(r); !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	cleanName := filepath.Base(fileName)
	if cleanName == "" || cleanName == "." {
		cleanName = "document.bin"
	}
	for _, document := range store.documents {
		if document.title != title {
			continue
		}
		document.edition = edition
		document.accessLevel = accessLevel
		document.fileName = cleanName
		document.contentType = contentType
		document.body = body
		writeJSON(w, http.StatusOK, uploadResponse{
			DocumentID: document.id,
			Status:     texts.StatusAccepted,
			Edition:    document.edition,
		})
		return
	}
	documentID := "doc-" + strconv.Itoa(store.nextDocument)
	store.nextDocument++
	store.addDocument(documentID, title, edition, accessLevel, cleanName, contentType, body)
	writeJSON(w, http.StatusOK, uploadResponse{
		DocumentID: documentID,
		Status:     texts.StatusAccepted,
		Edition:    edition,
	})
}

func (store *stub) serveDocumentFile(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	document := store.documentsByID[r.PathValue("documentId")]
	if document == nil {
		writeError(w, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if record.accessLevel < document.accessLevel {
		writeError(w, http.StatusForbidden, texts.ErrorAccessDenied)
		return
	}
	w.Header().Set("Content-Type", document.contentType)
	w.Header().Set("Content-Disposition", attachmentHeader(document.fileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(document.body)
}

func (store *stub) serveListUsers(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.adminFromRequest(w, r); !ok {
		return
	}
	accounts := make([]accountResponse, 0, len(store.users))
	for _, record := range store.users {
		accounts = append(accounts, record.account())
	}
	writeJSON(w, http.StatusOK, struct {
		Users []accountResponse `json:"users"`
	}{Users: accounts})
}

func (store *stub) serveCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login       string `json:"login"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
		AccessLevel *int   `json:"accessLevel"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	body.Login = strings.TrimSpace(body.Login)
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.Login == "" || body.Password == "" || body.DisplayName == "" {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	accessLevel := texts.SeedDocumentAccessOpen
	if body.AccessLevel != nil {
		accessLevel = *body.AccessLevel
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.adminFromRequest(w, r); !ok {
		return
	}
	if store.usersByLogin[body.Login] != nil {
		writeError(w, http.StatusConflict, texts.ErrorLoginTaken)
		return
	}
	userID := "user-" + strconv.Itoa(store.nextUser)
	store.nextUser++
	store.addUser(userID, body.Login, body.Password, body.DisplayName, accessLevel)
	writeJSON(w, http.StatusOK, store.usersByID[userID].account())
}

func (store *stub) servePatchUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName *string `json:"displayName"`
		AccessLevel *int    `json:"accessLevel"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	if body.DisplayName == nil && body.AccessLevel == nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.adminFromRequest(w, r); !ok {
		return
	}
	record := store.usersByID[r.PathValue("userId")]
	if record == nil {
		writeError(w, http.StatusNotFound, texts.ErrorNotFound)
		return
	}
	if body.DisplayName != nil {
		record.displayName = strings.TrimSpace(*body.DisplayName)
	}
	if body.AccessLevel != nil {
		record.accessLevel = *body.AccessLevel
	}
	writeJSON(w, http.StatusOK, record.account())
}

func (store *stub) serveAudit(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.adminFromRequest(w, r); !ok {
		return
	}
	entries := store.audit
	if entries == nil {
		entries = []auditEntry{}
	}
	writeJSON(w, http.StatusOK, struct {
		Entries []auditEntry `json:"entries"`
	}{Entries: entries})
}

func (store *stub) serveDraftCheck(w http.ResponseWriter, r *http.Request) {
	if _, _, _, err := readUpload(w, r); err != nil {
		writeError(w, http.StatusBadRequest, texts.ErrorInvalidBody)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.userFromRequest(r); !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, struct {
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
					OriginalLink: fileLink(texts.SeedInstructionID, texts.DraftParagraph),
				},
			},
		},
	})
}

func (store *stub) ownedSession(w http.ResponseWriter, r *http.Request) (*sessionRecord, bool) {
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return nil, false
	}
	session := store.sessions[r.PathValue("sessionId")]
	if session == nil {
		writeError(w, http.StatusNotFound, texts.ErrorNotFound)
		return nil, false
	}
	if session.ownerID != record.id {
		writeError(w, http.StatusForbidden, texts.ErrorAccessDenied)
		return nil, false
	}
	return session, true
}

func (store *stub) adminFromRequest(w http.ResponseWriter, r *http.Request) (*userRecord, bool) {
	record, ok := store.userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, texts.ErrorUnauthorized)
		return nil, false
	}
	if record.accessLevel < adminAccessLevel {
		writeError(w, http.StatusForbidden, texts.ErrorAccessDenied)
		return nil, false
	}
	return record, true
}

func (store *stub) userFromRequest(r *http.Request) (*userRecord, bool) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" || token != strings.TrimSpace(token) {
		return nil, false
	}
	userID := store.tokens[token]
	record := store.usersByID[userID]
	if record == nil {
		return nil, false
	}
	return record, true
}

func (store *stub) appendAudit(record *userRecord, question, kind string) {
	documents := []shownDocument{}
	if kind == texts.KindAnswer {
		documents = []shownDocument{{
			DocumentID: texts.SeedInstructionID,
			Title:      texts.SeedInstructionTitle,
			Paragraph:  texts.AnswerParagraph,
		}}
	}
	store.audit = append(store.audit, auditEntry{
		EntryID:     "audit-" + strconv.Itoa(store.nextAudit),
		UserID:      record.id,
		DisplayName: record.displayName,
		Question:    question,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Documents:   documents,
	})
	store.nextAudit++
}

func (record *userRecord) account() accountResponse {
	return accountResponse{
		UserID:      record.id,
		Login:       record.login,
		DisplayName: record.displayName,
		AccessLevel: record.accessLevel,
	}
}

func (document *documentRecord) public() documentResponse {
	return documentResponse{
		DocumentID:  document.id,
		Title:       document.title,
		Edition:     document.edition,
		AccessLevel: document.accessLevel,
	}
}

// Следующая реплика после вопроса по полю бланка завершает черновик.
func replyFor(session *sessionRecord, text string) assistantReply {
	if session.templateStep == templateStepAsked {
		session.templateStep = templateStepIdle
		return templateReadyReply()
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, texts.PhraseAccessDenied):
		return plainReply(texts.KindAccessDenied, texts.AccessDeniedText)
	case strings.Contains(lower, texts.PhraseNotFound):
		return plainReply(texts.KindFallback, texts.FallbackText)
	case strings.Contains(lower, texts.PhraseClarify):
		return plainReply(texts.KindClarify, texts.ClarifyText)
	case strings.Contains(lower, texts.PhraseTemplate):
		session.templateStep = templateStepAsked
		return templateFieldReply()
	default:
		return answerReply()
	}
}

func plainReply(kind, text string) assistantReply {
	return assistantReply{
		Kind:             kind,
		Text:             text,
		RelevancePercent: 0,
		Citations:        []citation{},
	}
}

func answerReply() assistantReply {
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
			OriginalLink: fileLink(texts.SeedInstructionID, texts.AnswerParagraph),
		}},
	}
}

func templateFieldReply() assistantReply {
	reply := plainReply(texts.KindTemplateField, texts.TemplateFieldText)
	reply.Template = &templateState{
		ID:         texts.TemplateID,
		Title:      texts.TemplateTitle,
		Field:      texts.TemplateField,
		FieldLabel: texts.TemplateFieldLabel,
	}
	return reply
}

func templateReadyReply() assistantReply {
	reply := plainReply(texts.KindTemplateReady, texts.TemplateReadyText)
	reply.Template = &templateState{
		ID:        texts.TemplateID,
		Title:     texts.TemplateTitle,
		DraftText: texts.TemplateDraftText,
	}
	return reply
}

func fileLink(documentID, paragraph string) string {
	return "/documents/" + documentID + "/file#p" + paragraph
}

func passwordMatches(stored, given string) bool {
	if len(stored) != len(given) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(given)) == 1
}

func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func readUpload(w http.ResponseWriter, r *http.Request) (string, []byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return "", nil, "", err
	}
	file, header, err := r.FormFile("file")
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

func attachmentHeader(name string) string {
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
