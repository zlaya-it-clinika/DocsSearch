package main

import (
	"log"
	"net/http"

	"docssearch/core/internal/config"
	"docssearch/core/internal/httpapi"
)

func main() {
	cfg := config.Load()
	if cfg.Debug {
		log.Printf("listen :%s mode %s ollama_configured %t", cfg.Port, cfg.Mode, cfg.OllamaURL != "")
	}
	err := http.ListenAndServe(":"+cfg.Port, httpapi.NewHandler(cfg))
	if err != nil {
		log.Fatal(err)
	}
}
