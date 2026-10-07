package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DrKaelum/Kurier/services/api/internal/ownership"
)

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		cloudMain()
		return
	}
	initLocal := flag.Bool("init-local", false, "initialize the loopback-only DynamoDB Local subset")
	cleanup := flag.String("cleanup-project", "", "resume empty-project deletion locally")
	flag.Parse()
	if *initLocal && *cleanup != "" {
		log.Fatal("choose only one local maintenance command")
	}
	client, err := ownership.LocalClient(os.Getenv("KURIER_DYNAMODB_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}
	table := envOrDefault("KURIER_CONTROL_TABLE", "kurier-local-control")
	stage := envOrDefault("KURIER_STAGE", "local")
	store := ownership.NewStore(client, table, stage)
	if *initLocal || *cleanup != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if *initLocal {
			if err = ownership.InitLocal(ctx, client, table, stage); err != nil {
				log.Fatal("local initialization failed")
			}
			log.Print("local subset initialized")
			return
		}
		op, e := store.CleanupProject(ctx, *cleanup)
		if e != nil {
			log.Fatal("local cleanup pending or unavailable; no completion claimed")
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"data": map[string]any{"deletionOperation": op}})
		return
	}
	handler, err := runtimeHandler(store, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	address := "127.0.0.1:" + envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 * 1024,
	}

	log.Printf("kurier api listening on %s", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runtimeHandler(store *ownership.Store, getenv func(string) string) (http.Handler, error) {
	verifier, err := ownership.NewCognitoVerifier(getenv("KURIER_COGNITO_ISSUER"), getenv("KURIER_COGNITO_CLIENT_ID"))
	if err != nil {
		return nil, err
	}
	secret, err := base64.StdEncoding.DecodeString(getenv("KURIER_CURSOR_KEY_BASE64"))
	if err != nil || len(secret) < 32 {
		return nil, errors.New("base64 cursor signing key of at least 32 bytes required")
	}
	handler, err := ownership.NewServer(store, verifier, secret)
	if err != nil {
		return nil, err
	}
	origin := getenv("KURIER_FRONTEND_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return ownership.LocalCORS(handler, origin)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
