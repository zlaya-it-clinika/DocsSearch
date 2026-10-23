package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	ModeStub = "stub"
	ModeLive = "live"
)

type Config struct {
	Debug                 bool
	Mode                  string
	Port                  string
	DocsPassword          string
	DatabaseURL           string
	OllamaURL             string
	OllamaChatModel       string
	OllamaEmbedModel      string
	OllamaEmbedDimensions int
}

func Load() Config {
	load_DotEnv(".env")
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("API_MODE")))
	if mode == "" {
		mode = ModeStub
	}
	port := strings.TrimPrefix(strings.TrimSpace(os.Getenv("API_PORT")), ":")
	if port == "" {
		port = "8080"
	}
	return Config{
		Debug:                 os.Getenv("DEBUG") == "true",
		Mode:                  mode,
		Port:                  port,
		DocsPassword:          os.Getenv("DOCS_PASSWORD"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		OllamaURL:             os.Getenv("OLLAMA_URL"),
		OllamaChatModel:       os.Getenv("OLLAMA_CHAT_MODEL"),
		OllamaEmbedModel:      os.Getenv("OLLAMA_EMBED_MODEL"),
		OllamaEmbedDimensions: read_EmbedDimensions(),
	}
}

func read_EmbedDimensions() int {
	raw := strings.TrimSpace(os.Getenv("OLLAMA_EMBED_DIMENSIONS"))
	if raw == "" {
		return 768
	}
	parsed, parseError := strconv.Atoi(raw)
	if parseError != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func load_DotEnv(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
