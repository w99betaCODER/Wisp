// Command wisp-certs generates the small PKI for the panel↔node mTLS link:
// a CA, a server certificate for the node agent, and a client certificate for
// the panel — all signed by the CA. Run it once, then distribute the files.
//
//	go run ./cmd/wisp-certs -dir certs -host node1.example.com,1.2.3.4
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dir := flag.String("dir", "certs", "output directory")
	hosts := flag.String("host", "localhost,127.0.0.1", "comma-separated DNS names / IPs for the node server cert")
	years := flag.Int("years", 10, "certificate validity in years")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}

	notBefore := time.Now().Add(-time.Hour)
	notAfter := notBefore.AddDate(*years, 0, 0)

	// --- Certificate Authority ---
	caKey := genKey()
	caTmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "Wisp CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER := create(caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, err := x509.ParseCertificate(caDER)
	must(err)
	writeCert(filepath.Join(*dir, "ca.crt"), caDER)
	writeKey(filepath.Join(*dir, "ca.key"), caKey)

	// --- Server certificate (node agent) ---
	srvKey := genKey()
	srvTmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "wisp-node"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range splitHosts(*hosts) {
		if ip := net.ParseIP(h); ip != nil {
			srvTmpl.IPAddresses = append(srvTmpl.IPAddresses, ip)
		} else {
			srvTmpl.DNSNames = append(srvTmpl.DNSNames, h)
		}
	}
	srvDER := create(srvTmpl, caCert, &srvKey.PublicKey, caKey)
	writeCert(filepath.Join(*dir, "server.crt"), srvDER)
	writeKey(filepath.Join(*dir, "server.key"), srvKey)

	// --- Client certificate (panel) ---
	cliKey := genKey()
	cliTmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "wisp-panel"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	cliDER := create(cliTmpl, caCert, &cliKey.PublicKey, caKey)
	writeCert(filepath.Join(*dir, "client.crt"), cliDER)
	writeKey(filepath.Join(*dir, "client.key"), cliKey)

	fmt.Printf("wrote ca / server / client certs to %s/\n\n", *dir)
	fmt.Println("node agent env:")
	fmt.Println("  WISP_TLS_CERT=server.crt WISP_TLS_KEY=server.key WISP_TLS_CLIENT_CA=ca.crt")
	fmt.Println("panel env:")
	fmt.Println("  WISP_NODE_TLS_CERT=client.crt WISP_NODE_TLS_KEY=client.key WISP_NODE_TLS_CA=ca.crt")
}

func genKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	return key
}

func create(tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, signer *ecdsa.PrivateKey) []byte {
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	must(err)
	return der
}

func splitHosts(s string) []string {
	var out []string
	for _, h := range strings.Split(s, ",") {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	must(err)
	return n
}

func writeCert(path string, der []byte) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func writeKey(path string, key *ecdsa.PrivateKey) {
	der, err := x509.MarshalECPrivateKey(key)
	must(err)
	// Private keys must never be world-readable.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	must(err)
	defer f.Close()
	must(pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
