package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"sync"

	"docssearch/core/internal/texts"
)

//go:embed docs.html
var docsFiles embed.FS

var docsTemplates = template.Must(template.ParseFS(docsFiles, "docs.html"))

const docsCookieName = "docs_session"

type docsGate struct {
	password string
	mu       sync.Mutex
	sessions map[string]struct{}
}

type docsLoginView struct {
	Error       string
	PasswordSet bool
}

func newDocsGate(password string) *docsGate {
	return &docsGate{
		password: password,
		sessions: map[string]struct{}{},
	}
}

func registerDocs(mux *http.ServeMux, gate *docsGate) {
	mux.HandleFunc("GET /docs", gate.servePage)
	mux.HandleFunc("POST /docs/login", gate.serveLogin)
	mux.HandleFunc("POST /docs/logout", gate.serveLogout)
}

func (gate *docsGate) servePage(w http.ResponseWriter, r *http.Request) {
	if !gate.allowed(r) {
		renderDocs(w, "login", docsLoginView{
			Error:       gate.lockedMessage(),
			PasswordSet: gate.password != "",
		}, http.StatusOK)
		return
	}
	renderDocs(w, "console", nil, http.StatusOK)
}

func (gate *docsGate) serveLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderDocs(w, "login", docsLoginView{
			Error:       texts.DocsBadForm,
			PasswordSet: gate.password != "",
		}, http.StatusBadRequest)
		return
	}
	if gate.password == "" {
		renderDocs(w, "login", docsLoginView{
			Error:       texts.DocsPasswordNotSet,
			PasswordSet: false,
		}, http.StatusServiceUnavailable)
		return
	}
	if !passwordMatches(gate.password, r.FormValue("password")) {
		renderDocs(w, "login", docsLoginView{
			Error:       texts.DocsWrongPassword,
			PasswordSet: true,
		}, http.StatusUnauthorized)
		return
	}
	token, err := newToken()
	if err != nil {
		renderDocs(w, "login", docsLoginView{
			Error:       texts.DocsBadForm,
			PasswordSet: true,
		}, http.StatusInternalServerError)
		return
	}
	gate.mu.Lock()
	gate.sessions[token] = struct{}{}
	gate.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     docsCookieName,
		Value:    token,
		Path:     "/docs",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/docs", http.StatusSeeOther)
}

func (gate *docsGate) serveLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(docsCookieName); err == nil {
		gate.mu.Lock()
		delete(gate.sessions, cookie.Value)
		gate.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     docsCookieName,
		Value:    "",
		Path:     "/docs",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/docs", http.StatusSeeOther)
}

func (gate *docsGate) allowed(r *http.Request) bool {
	cookie, err := r.Cookie(docsCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	_, ok := gate.sessions[cookie.Value]
	return ok
}

// Пустой DOCS_PASSWORD не должен открывать страницу: сравнение двух пустых строк дало бы совпадение.
func (gate *docsGate) lockedMessage() string {
	if gate.password == "" {
		return texts.DocsPasswordNotSet
	}
	return ""
}

func renderDocs(w http.ResponseWriter, name string, data any, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = docsTemplates.ExecuteTemplate(w, name, data)
}
