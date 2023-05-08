package main

import (
	"github.com/CloudNativeGame/fake-time-injector/pkg/webhook"
	"log"
	"net/http"
)

func main() {
	var wo *webhook.WebHookOptions
	var err error
	if wo, err = webhook.NewWebHookOptions(); err != nil {
		log.Fatalf("Please input valid params. %v", err)
	}

	ws, err := webhook.NewWebHookServer(wo)

	if err != nil {
		log.Fatalf("Failed to set up webhook server: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(webhook.MutatingWebhookConfigurationPath, ws.Serve)
	ws.Server.Handler = mux

	log.Fatal(ws.Run())
}
