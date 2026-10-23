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

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func New_Client(baseURL, model string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:      strings.TrimSpace(model),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (client *Client) Embed_Texts(inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if client == nil || client.baseURL == "" || client.model == "" {
		return nil, ErrUnavailable
	}
	payload, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: client.model, Input: inputs})
	if err != nil {
		return nil, ErrUnavailable
	}
	request, err := http.NewRequest(http.MethodPost, client.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
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
