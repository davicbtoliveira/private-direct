package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davicbtoliveira/private-direct/internal/app"
)

func main() {
	cfg := app.Config{
		Addr:                 envOrDefault("PRIVATE_DIRECT_ADDR", ":8080"),
		DatabasePath:         envOrDefault("PRIVATE_DIRECT_DB", "private-direct.db"),
		OperatorToken:        os.Getenv("PRIVATE_DIRECT_OPERATOR_TOKEN"),
		JWTSecret:            os.Getenv("PRIVATE_DIRECT_JWT_SECRET"),
		PwnedPasswordsURL:    "https://api.pwnedpasswords.com/range",
		MessageQuotaBytes:    envInt64("PRIVATE_DIRECT_MESSAGE_QUOTA_BYTES", 100*1024*1024),
		MessageRatePerMinute: int(envInt64("PRIVATE_DIRECT_MESSAGE_RATE_PER_MINUTE", 120)),
		MessageRateBurst:     int(envInt64("PRIVATE_DIRECT_MESSAGE_RATE_BURST", 30)),
		STUNServers:          splitCSV(os.Getenv("PRIVATE_DIRECT_STUN_URLS")),
		TURNServers: []app.ICEServer{
			{
				URLs:       splitCSV(os.Getenv("PRIVATE_DIRECT_TURN_URLS")),
				Username:   os.Getenv("PRIVATE_DIRECT_TURN_USERNAME"),
				Credential: os.Getenv("PRIVATE_DIRECT_TURN_CREDENTIAL"),
			},
		},
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

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	var items []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
