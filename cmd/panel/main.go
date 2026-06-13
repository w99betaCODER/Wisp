// Command panel is the Wisp control-plane server.
//
// It exposes a small HTTP API to manage VPN users and (later) talks to
// Xray nodes over gRPC. For now it runs with an in-memory store so it
// starts instantly with zero dependencies — persistence comes next.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wisp-panel/wisp/internal/config"
	"github.com/wisp-panel/wisp/internal/server"
	"github.com/wisp-panel/wisp/internal/store"
)

func main() {
	cfg := config.Load()

	// For now the data lives in memory. Swapping this for a SQLite-backed
	// store later changes only this one line — everything else depends on
	// the store.Store interface, not the concrete type.
	st := store.NewMemoryStore()

	srv := server.New(cfg, st)

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      srv.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Run the server in a goroutine so main can wait for shutdown signals.
	go func() {
		log.Printf("wisp panel listening on %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until the OS asks us to stop (Ctrl+C or `docker stop`).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Give in-flight requests up to 5s to finish before exiting.
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
