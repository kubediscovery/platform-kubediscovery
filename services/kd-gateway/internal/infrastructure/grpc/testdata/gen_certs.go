//go:build ignore

// gen_certs.go generates the test CA, server and client certificates used by
// the grpc server tests.  Run it once:
//
//	go run testdata/gen_certs.go
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	caKey, caCert := genCA()
	writeKey("ca.key", caKey)
	writeCert("ca.crt", caCert)

	srvKey, srvCert := genCert(caKey, caCert, "server", nil, []net.IP{net.ParseIP("127.0.0.1")})
	writeKey("server.key", srvKey)
	writeCert("server.crt", srvCert)

	clientKey, clientCert := genCert(caKey, caCert, "client", nil, nil)
	writeKey("client.key", clientKey)
	writeCert("client.crt", clientCert)
}

func genCA() (*ecdsa.PrivateKey, *x509.Certificate) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

func genCert(caKey *ecdsa.PrivateKey, caCert *x509.Certificate, cn string, dns []string, ips []net.IP) (*ecdsa.PrivateKey, *x509.Certificate) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		DNSNames:     dns,
		IPAddresses:  ips,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

func writeKey(name string, key *ecdsa.PrivateKey) {
	b, _ := x509.MarshalECPrivateKey(key)
	f, _ := os.Create(name)
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
}

func writeCert(name string, cert *x509.Certificate) {
	f, _ := os.Create(name)
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}
