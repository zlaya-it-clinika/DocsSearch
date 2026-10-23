package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"docssearch/core/internal/config"
	"docssearch/core/internal/livestore"
	"docssearch/core/internal/ollama"
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

func NewHandler(loadedConfig config.Config) (http.Handler, error) {
	var routeMux *http.ServeMux
	if strings.EqualFold(loadedConfig.Mode, config.ModeLive) {
		if strings.TrimSpace(loadedConfig.DatabaseURL) == "" {
			return nil, errors.New(texts.LiveDatabaseMissing)
		}
		embedder := ollama.New_Client(loadedConfig.OllamaURL, loadedConfig.OllamaEmbedModel, loadedConfig.OllamaChatModel)
		store, err := livestore.Open_LiveStore(loadedConfig.DatabaseURL, loadedConfig.OllamaEmbedDimensions, embedder)
		if err != nil {
			return nil, err
		}
		routeMux = register_LiveRoutes(store, embedder)
	} else {
		routeMux = create_StubStore().register_StubRoutes()
	}
	register_DocsRoutes(routeMux, create_DocsGate(loadedConfig.DocsPassword))
	return log_RequestsWhenDebug(loadedConfig.Debug, allow_CrossOrigin(routeMux)), nil
}

func allow_CrossOrigin(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
		responseWriter.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if request.Method == http.MethodOptions {
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		nextHandler.ServeHTTP(responseWriter, request)
	})
}

func log_RequestsWhenDebug(enabled bool, nextHandler http.Handler) http.Handler {
	if !enabled {
		return nextHandler
	}
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		recorder := &statusWriter{ResponseWriter: responseWriter, status: http.StatusOK}
		nextHandler.ServeHTTP(recorder, request)
		log.Printf("%s %s %d", request.Method, request.URL.Path, recorder.status)
	})
}

func write_JSONResponse(responseWriter http.ResponseWriter, status int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func write_ErrorResponse(responseWriter http.ResponseWriter, status int, code string) {
	write_JSONResponse(responseWriter, status, errorBody{Error: code})
}

func read_JSONBody(responseWriter http.ResponseWriter, request *http.Request, decodeTarget any) error {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxJSONBytes)
	return json.NewDecoder(request.Body).Decode(decodeTarget)
}
