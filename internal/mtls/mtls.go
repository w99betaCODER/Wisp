// Package mtls builds mutual-TLS configurations for the panel↔node link.
//
// Both sides present a certificate signed by a shared CA and verify the other
// against that CA, so only a panel holding a valid client cert can drive a
// node, and the panel only trusts nodes presenting a valid server cert.
package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// ServerConfig builds a tls.Config for the node agent: it presents
// certFile/keyFile and requires the caller to present a client certificate
// signed by clientCAFile.
func ServerConfig(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}
	pool, err := loadCAPool(clientCAFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// ClientConfig builds a tls.Config for the panel: it presents certFile/keyFile
// and verifies the node's server certificate against caFile.
func ClientConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client keypair: %w", err)
	}
	pool, err := loadCAPool(caFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// loadCAPool reads a PEM bundle into a fresh certificate pool.
func loadCAPool(caFile string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates found in %s", caFile)
	}
	return pool, nil
}
