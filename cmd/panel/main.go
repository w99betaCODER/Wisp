// Command panel is the Wisp control-plane server.
//
// It exposes a small HTTP API to manage VPN users and (later) talks to
// Xray nodes over gRPC. For now it runs with an in-memory store so it
// starts instantly with zero dependencies — persistence comes next.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/enforcer"
	"github.com/w99betaCODER/Wisp/internal/mtls"
	"github.com/w99betaCODER/Wisp/internal/server"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

func main() {
	cfg := config.Load()

	// Persist users in SQLite. Because the rest of the app depends on the
	// store.Store interface (not the concrete type), this is the only line
	// that knows we use SQLite at all.
	st, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Seed (or refresh) the super-admin from WISP_ADMIN_USER / WISP_ADMIN_PASS.
	// With no password set the panel runs open (dev mode).
	if err := server.Bootstrap(st, cfg); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	// Connect to Xray if configured; otherwise fall back to a no-op client so
	// the panel still runs (users are stored but not pushed to a proxy).
	xc := newXrayClient(cfg)
	defer xc.Close()

	// Build the panel's mTLS client config for talking to node agents. Without
	// it the panel reaches nodes over plain HTTP — fine for local development,
	// not for production.
	var clusterTLS *tls.Config
	if cfg.ClientCert != "" {
		clusterTLS, err = mtls.ClientConfig(cfg.ClientCert, cfg.ClientKey, cfg.ClientCA)
		if err != nil {
			log.Fatalf("node mTLS config: %v", err)
		}
		log.Println("node mTLS enabled")
	} else {
		log.Println("WISP_NODE_TLS_CERT not set — talking to nodes over plain HTTP (dev only)")
	}
	cl := cluster.New(st, clusterTLS)

	// Start the background enforcer: it accounts traffic and disables users
	// that exceed their quota or expire. It stops when enfCancel is called.
	enfCtx, enfCancel := context.WithCancel(context.Background())
	defer enfCancel()
	go enforcer.New(st, xc, cl, cfg).Run(enfCtx)

	srv := server.New(cfg, st, xc, cl)

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

// newXrayClient returns a real gRPC client when WISP_XRAY_API is configured,
// or a no-op client otherwise so the panel runs without an Xray instance.
func newXrayClient(cfg config.Config) xray.Client {
	if cfg.XrayAPIAddr == "" {
		log.Println("WISP_XRAY_API not set — using no-op Xray client (users are stored but not pushed to Xray)")
		return xray.NewNoopClient()
	}
	gc, err := xray.Dial(cfg.XrayAPIAddr, cfg.Protocol)
	if err != nil {
		log.Fatalf("connect to xray: %v", err)
	}
	log.Printf("connected to xray API at %s (protocol %s)", cfg.XrayAPIAddr, cfg.Protocol)
	return gc
}
