package main

import (
	"log"
	"net/http"

	"docssearch/core/internal/config"
	"docssearch/core/internal/httpapi"
)

func main() {
	loadedConfig := config.Load()
	if loadedConfig.Debug {
		log.Printf("listen :%s mode %s ollama_configured %t", loadedConfig.Port, loadedConfig.Mode, loadedConfig.OllamaURL != "")
	}
	listenError := http.ListenAndServe(":"+loadedConfig.Port, httpapi.NewHandler(loadedConfig))
	if listenError != nil {
		log.Fatal(listenError)
	}
}
