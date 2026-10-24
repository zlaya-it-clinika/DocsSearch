package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

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
	var routeHandler http.Handler
	if strings.EqualFold(loadedConfig.Mode, config.ModeLive) {
		if strings.TrimSpace(loadedConfig.DatabaseURL) == "" {
			return nil, errors.New(texts.LiveDatabaseMissing)
		}
		embedder := ollama.New_Client(loadedConfig.OllamaURL, loadedConfig.OllamaEmbedModel, loadedConfig.OllamaChatModel)
		store, err := livestore.Open_LiveStore(loadedConfig.DatabaseURL, loadedConfig.OllamaEmbedDimensions, embedder, loadedConfig.AuthDefaults)
		if err != nil {
			return nil, err
		}
		apply_SavedAISettings(store, embedder)
		routeMux = register_LiveRoutes(store, embedder)
		register_AIRoutes(routeMux, create_LiveAIAdmin(store, embedder))
		register_OrganizationRoutes(routeMux, store)
		routeHandler = routeMux
	} else {
		stubStore := create_StubStore(loadedConfig.StubWithoutUsers, loadedConfig.AuthDefaults)
		routeMux = stubStore.register_StubRoutes()
		client := ollama.New_Client(loadedConfig.OllamaURL, loadedConfig.OllamaEmbedModel, loadedConfig.OllamaChatModel)
		register_AIRoutes(routeMux, stubStore.create_AIAdmin(client, loadedConfig.OllamaEmbedDimensions))
		register_OrganizationRoutes(routeMux, stubStore.create_Organization())
		// В боевом режиме ожидающих отсекает хранилище, в заглушке - эта обёртка
		routeHandler = stubStore.reject_PendingUsers(routeMux)
	}
	register_DocsRoutes(routeMux, create_DocsGate(loadedConfig.DocsPassword))
	return log_RequestsWhenDebug(loadedConfig.Debug, allow_CrossOrigin(routeHandler)), nil
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

// Настройки из админ-панели важнее .env. Пока их не сохраняли, клиент работает со значениями из .env
func apply_SavedAISettings(store *livestore.LiveStore, client *ollama.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, found, err := store.Read_SavedAISettings(ctx)
	if err != nil {
		log.Printf("ai settings: %v", err)
		return
	}
	if found {
		client.Apply_Settings(saved)
	}
}

func create_LiveAIAdmin(store *livestore.LiveStore, client *ollama.Client) *aiAdmin {
	return &aiAdmin{
		backend:    client,
		dimensions: store.Dimensions(),
		authorize: func(ctx context.Context, token string) error {
			return store.Require_Permission(ctx, token, texts.PermissionAIManage)
		},
		persist:   store.Save_AISettings,
		reindexer: store,
	}
}
