package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davicbtoliveira/private-direct/internal/app"
)

func main() {
	cfg := app.Config{
		Addr:          envOrDefault("PRIVATE_DIRECT_ADDR", ":8080"),
		DatabasePath:  envOrDefault("PRIVATE_DIRECT_DB", "private-direct.db"),
		OperatorToken: os.Getenv("PRIVATE_DIRECT_OPERATOR_TOKEN"),
		JWTSecret:     os.Getenv("PRIVATE_DIRECT_JWT_SECRET"),
	}

	srv, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.Addr)
		errs <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	case <-stop:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
