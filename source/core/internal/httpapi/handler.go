package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"docssearch/core/internal/config"
	"docssearch/core/internal/texts"
)

const maxJSONBytes = 1 << 20

type errorBody struct {
	Error string `json:"error"`
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (writer *statusWriter) WriteHeader(code int) {
	if writer.wrote {
		return
	}
	writer.status = code
	writer.wrote = true
	writer.ResponseWriter.WriteHeader(code)
}

func (writer *statusWriter) Write(payload []byte) (int, error) {
	if !writer.wrote {
		writer.status = http.StatusOK
		writer.wrote = true
	}
	return writer.ResponseWriter.Write(payload)
}

func NewHandler(cfg config.Config) http.Handler {
	var mux *http.ServeMux
	if strings.EqualFold(cfg.Mode, config.ModeLive) {
		mux = http.NewServeMux()
		mux.HandleFunc("/", serveLive)
	} else {
		mux = newStub().routes()
	}
	registerDocs(mux, newDocsGate(cfg.DocsPassword))
	return debugRequests(cfg.Debug, allowCORS(mux))
}

func serveLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, errorBody{Error: texts.ErrorLiveNotReady})
}

func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func debugRequests(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, recorder.status)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Error: code})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}
