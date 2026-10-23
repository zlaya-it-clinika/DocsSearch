package ollama

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("embed unavailable")

const (
	embedTimeout = 60 * time.Second
	chatTimeout  = 120 * time.Second
)

type Client struct {
	baseURL     string
	embedModel  string
	chatModel   string
	embedClient *http.Client
	chatClient  *http.Client
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func New_Client(baseURL, embedModel, chatModel string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		embedModel:  strings.TrimSpace(embedModel),
		chatModel:   strings.TrimSpace(chatModel),
		embedClient: &http.Client{Timeout: embedTimeout},
		chatClient:  &http.Client{Timeout: chatTimeout},
	}
}

func (client *Client) Embed_Texts(inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if client == nil || client.baseURL == "" || client.embedModel == "" {
		return nil, ErrUnavailable
	}
	payload, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: client.embedModel, Input: inputs})
	if err != nil {
		return nil, ErrUnavailable
	}
	request, err := http.NewRequest(http.MethodPost, client.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.embedClient.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil || response.StatusCode >= 300 {
		return nil, ErrUnavailable
	}
	var parsed struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, ErrUnavailable
	}
	if len(parsed.Embeddings) != len(inputs) {
		return nil, ErrUnavailable
	}
	return parsed.Embeddings, nil
}

func (client *Client) Complete_Chat(messages []ChatMessage) (string, error) {
	if client == nil || client.baseURL == "" || client.chatModel == "" || len(messages) == 0 {
		return "", ErrUnavailable
	}
	payload, err := json.Marshal(struct {
		Model    string        `json:"model"`
		Messages []ChatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
	}{Model: client.chatModel, Messages: messages, Stream: false})
	if err != nil {
		return "", ErrUnavailable
	}
	request, err := http.NewRequest(http.MethodPost, client.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.chatClient.Do(request)
	if err != nil {
		return "", ErrUnavailable
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil || response.StatusCode >= 300 {
		return "", ErrUnavailable
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", ErrUnavailable
	}
	if strings.TrimSpace(parsed.Message.Content) == "" {
		return "", ErrUnavailable
	}
	return parsed.Message.Content, nil
}
