package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serverCertificatePEM(t *testing.T, srv *httptest.Server) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}

func doNoContent(t *testing.T, cfg Config) error {
	t.Helper()
	client, err := New(cfg)
	if err != nil {
		return err
	}
	return client.Do(context.Background(), "tls probe", http.MethodGet, "/", nil, nil)
}

func TestNew_ServerTrust(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	t.Cleanup(srv.Close)
	ca := serverCertificatePEM(t, srv)
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"untrusted by default", Config{BaseURL: srv.URL, Attempts: 1}, true},
		{"supplied CA", Config{BaseURL: srv.URL, CACertificate: ca, Attempts: 1}, false},
		{"explicit insecure", Config{BaseURL: srv.URL, InsecureSkipVerify: true, Attempts: 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := doNoContent(t, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestNew_InvalidCA(t *testing.T) {
	t.Parallel()
	_, err := New(Config{BaseURL: "https://example.invalid", CACertificate: []byte("not PEM")})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func newClientIdentity(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	now := time.Now().UTC()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey CA: %v", err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublic, caPrivate)
	if err != nil {
		t.Fatalf("CreateCertificate CA: %v", err)
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey leaf: %v", err)
	}
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, leafPublic, caPrivate)
	if err != nil {
		t.Fatalf("CreateCertificate leaf: %v", err)
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPKCS8, err := x509.MarshalPKCS8PrivateKey(leafPrivate)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	identity, err := tls.X509KeyPair(leafPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyPKCS8}))
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("ParseCertificate CA: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCertificate)
	return identity, pool
}

func TestNew_MutualTLS(t *testing.T) {
	t.Parallel()
	identity, clientCAs := newClientIdentity(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	without := Config{BaseURL: srv.URL, InsecureSkipVerify: true, Attempts: 1}
	if err := doNoContent(t, without); err == nil {
		t.Fatal("handshake without a client certificate succeeded")
	}
	with := without
	with.ClientCert = &identity
	if err := doNoContent(t, with); err != nil {
		t.Fatalf("handshake with a client certificate: %v", err)
	}
}

func TestNew_TLSVersionFloor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		version uint16
		wantErr bool
	}{{"TLS 1.1 rejected", tls.VersionTLS11, true}, {"TLS 1.2 accepted", tls.VersionTLS12, false}, {"TLS 1.3 accepted", tls.VersionTLS13, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			srv.TLS = &tls.Config{MinVersion: tt.version, MaxVersion: tt.version}
			srv.StartTLS()
			t.Cleanup(srv.Close)
			err := doNoContent(t, Config{BaseURL: srv.URL, CACertificate: serverCertificatePEM(t, srv), Attempts: 1})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
