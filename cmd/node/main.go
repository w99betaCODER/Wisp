// Command node is the Wisp node agent. It runs on a VPN server next to Xray,
// accepts user operations from the panel over mTLS, and applies them to the
// local Xray instance via its gRPC API.
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

	"github.com/w99betaCODER/Wisp/internal/agent"
	"github.com/w99betaCODER/Wisp/internal/mtls"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

func main() {
	var (
		listen     = env("WISP_AGENT_LISTEN", ":8443")
		xrayAPI    = env("WISP_XRAY_API", "")
		inboundTag = env("WISP_INBOUND_TAG", "vless-reality")
		protocol   = env("WISP_PROTOCOL", "vless")
		tlsCert    = env("WISP_TLS_CERT", "")
		tlsKey     = env("WISP_TLS_KEY", "")
		clientCA   = env("WISP_TLS_CLIENT_CA", "")
	)

	xc, err := xray.New(xrayAPI, protocol)
	if err != nil {
		log.Fatalf("connect to xray: %v", err)
	}
	defer xc.Close()
	if xrayAPI == "" {
		log.Println("WISP_XRAY_API not set — node agent using no-op Xray client")
	}

	a := agent.New(xc, inboundTag)
	httpServer := &http.Server{
		Addr:         listen,
		Handler:      a.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// mTLS when certs are provided; plain HTTP otherwise (development only).
	useTLS := tlsCert != ""
	if useTLS {
		tlsCfg, err := mtls.ServerConfig(tlsCert, tlsKey, clientCA)
		if err != nil {
			log.Fatalf("tls config: %v", err)
		}
		httpServer.TLSConfig = tlsCfg
	}

	go func() {
		var err error
		if useTLS {
			log.Printf("wisp node agent listening on %s (mTLS)", listen)
			// Certs are already in TLSConfig, so the file args are empty.
			err = httpServer.ListenAndServeTLS("", "")
		} else {
			log.Printf("wisp node agent listening on %s (PLAIN HTTP — dev only)", listen)
			err = httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// env returns the value of key, or def if it is unset or empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
