package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Certificate creates a persistent self-signed LAN certificate. The private key is 0600.
func Certificate(dir, host string) (string, string, error) {
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if certBytes, e := os.ReadFile(certPath); e == nil {
		b, _ := pem.Decode(certBytes)
		if b != nil {
			cert, e := x509.ParseCertificate(b.Bytes)
			if e == nil && cert.VerifyHostname(host) == nil && time.Now().Add(24*time.Hour).Before(cert.NotAfter) {
				if _, e = os.Stat(keyPath); e == nil {
					return certPath, keyPath, nil
				}
			}
		}
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", "", e
	}
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return "", "", e
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return "", "", e
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "AURGA LAN viewer"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}
	der, e := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if e != nil {
		return "", "", e
	}
	keyBytes, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return "", "", e
	}
	// Atomic replacement prevents partial files after a crash; no credentials enter logs.
	for _, f := range []struct {
		path string
		data []byte
	}{{keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})}, {certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}} {
		tmp, e := os.CreateTemp(dir, ".tls-")
		if e != nil {
			return "", "", e
		}
		name := tmp.Name()
		_, e = tmp.Write(f.data)
		if e == nil {
			e = tmp.Sync()
		}
		closeErr := tmp.Close()
		if e == nil {
			e = closeErr
		}
		if e == nil {
			e = os.Rename(name, f.path)
		}
		if e != nil {
			os.Remove(name)
			return "", "", fmt.Errorf("store TLS certificate: %w", e)
		}
	}
	return certPath, keyPath, nil
}
