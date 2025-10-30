package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/health"
)

func main() {
	// Load configuration from environment
	cfg := config.Load()

	// Create router
	r := chi.NewRouter()

	// Health check endpoints
	r.Get("/health", health.Handler())
	r.Get("/ready", health.ReadyHandler())

	// Basic info endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":"fermi-api-gateway","version":"1.0.0","env":"%s"}`, cfg.Server.Env)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors coming from the listener.
	serverErrors := make(chan error, 1)

	// Start the server
	go func() {
		log.Printf("Starting API Gateway on port %s (env: %s)", cfg.Server.Port, cfg.Server.Env)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case sig := <-shutdown:
		log.Printf("Received signal %v, starting graceful shutdown", sig)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}

		log.Println("Server stopped gracefully")
	}
}
