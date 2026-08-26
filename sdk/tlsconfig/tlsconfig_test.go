package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientConfigInlinePEM(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	cfg, err := ClientConfig(&ClientOptions{
		CA:         &Material{InlinePEM: testCertPEM},
		Cert:       &Material{InlinePEM: testCertPEM},
		Key:        &Material{InlinePEM: testKeyPEM},
		ServerName: "localhost",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.RootCAs == nil {
		t.Fatal("expected root CAs")
	}
	if cfg.ServerName != "localhost" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d", len(cfg.Certificates))
	}
}

func TestClientConfigPathPEM(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.pem")
	keyPath := filepath.Join(dir, "client.key")
	err := os.WriteFile(certPath, []byte(testCertPEM), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(keyPath, []byte(testKeyPEM), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	var cfg *tls.Config
	cfg, err = ClientConfig(&ClientOptions{
		CA:   &Material{Path: certPath},
		Cert: &Material{Path: certPath},
		Key:  &Material{Path: keyPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatal("expected certificate")
	}
}

func TestClientConfigRejectsDualSource(t *testing.T) {
	testCertPEM, _ := selfSignedPEM(t)
	_, err := ClientConfig(&ClientOptions{
		CA: &Material{Path: "/tmp/ca.pem", InlinePEM: testCertPEM},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientConfigRejectsCertWithoutKey(t *testing.T) {
	testCertPEM, _ := selfSignedPEM(t)
	_, err := ClientConfig(&ClientOptions{
		Cert: &Material{InlinePEM: testCertPEM},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServerConfigSupportsClientAuthModes(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	tests := []struct {
		name string
		mode ClientAuthMode
		want tls.ClientAuthType
	}{
		{name: "default", want: tls.NoClientCert},
		{name: "none", mode: ClientAuthNone, want: tls.NoClientCert},
		{name: "verify if given", mode: ClientAuthVerifyIfGiven, want: tls.VerifyClientCertIfGiven},
		{name: "require and verify", mode: ClientAuthRequireAndVerify, want: tls.RequireAndVerifyClientCert},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ServerConfig(&ServerOptions{
				Cert:       &Material{InlinePEM: testCertPEM},
				Key:        &Material{InlinePEM: testKeyPEM},
				ClientCA:   &Material{InlinePEM: testCertPEM},
				ClientAuth: tc.mode,
			})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ClientAuth != tc.want {
				t.Fatalf("ClientAuth = %v, want %v", cfg.ClientAuth, tc.want)
			}
			if tc.want != tls.NoClientCert && cfg.ClientCAs == nil {
				t.Fatal("expected client CAs")
			}
		})
	}
}

func TestServerConfigRejectsUnknownClientAuth(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	mode := ClientAuthMode("require-and-verify")
	_, err := ServerConfig(&ServerOptions{
		Cert:       &Material{InlinePEM: testCertPEM},
		Key:        &Material{InlinePEM: testKeyPEM},
		ClientCA:   &Material{InlinePEM: testCertPEM},
		ClientAuth: mode,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), string(mode)) {
		t.Fatalf("error %q does not contain mode %q", err, mode)
	}
}

func TestServerConfigRejectsMissingClientCA(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	_, err := ServerConfig(&ServerOptions{
		Cert:       &Material{InlinePEM: testCertPEM},
		Key:        &Material{InlinePEM: testKeyPEM},
		ClientAuth: ClientAuthVerifyIfGiven,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServerConfigAppendsChainCA(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	cfg, err := ServerConfig(&ServerOptions{
		Cert: &Material{InlinePEM: testCertPEM},
		Key:  &Material{InlinePEM: testKeyPEM},
		CA:   &Material{InlinePEM: testCertPEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatal("expected certificate")
	}
	if len(cfg.Certificates[0].Certificate) == 0 {
		t.Fatal("expected parsed certificate chain")
	}
}

func TestMaterialRoundTrip(t *testing.T) {
	testCertPEM, _ := selfSignedPEM(t)
	src := &Material{InlinePEM: testCertPEM}
	pb := MaterialToProto(src)
	got := MaterialFromProto(pb)
	if got == nil || got.InlinePEM != src.InlinePEM {
		t.Fatalf("got = %#v", got)
	}
}

func TestClientConfigLoadsCertPool(t *testing.T) {
	testCertPEM, _ := selfSignedPEM(t)
	cfg, err := ClientConfig(&ClientOptions{CA: &Material{InlinePEM: testCertPEM}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected root CAs")
	}
	expected := x509.NewCertPool()
	ok := expected.AppendCertsFromPEM([]byte(testCertPEM))
	if !ok {
		t.Fatal("append expected CA")
	}
	if !cfg.RootCAs.Equal(expected) {
		t.Fatal("root CA pool does not contain the configured CA")
	}
}

func TestServerConfigProducesValidCertChain(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	cfg, err := ServerConfig(&ServerOptions{
		Cert: &Material{InlinePEM: testCertPEM},
		Key:  &Material{InlinePEM: testKeyPEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	cert := cfg.Certificates[0]
	_, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
}

func selfSignedPEM(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	var der []byte

	der, err = x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	var keyDER []byte

	keyDER, err = x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM)
}
