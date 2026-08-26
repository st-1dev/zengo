package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	metacfg "zengo/platform/api/config/meta"
)

// Material identifies one TLS artifact either by path or by inline PEM.
type Material struct {
	// Path points to a PEM file on disk.
	Path string
	// InlinePEM stores PEM content directly in config.
	InlinePEM string
}

// ClientOptions configures client-side TLS and optional mTLS.
type ClientOptions struct {
	// CA contains the CA bundle used to verify the remote server certificate.
	CA *Material
	// Cert contains the client certificate used for mTLS.
	Cert *Material
	// Key contains the private key that matches Cert.
	Key *Material
	// ServerName overrides the expected TLS server name.
	ServerName string
	// InsecureSkipVerify disables remote certificate verification.
	InsecureSkipVerify bool
}

// ClientAuthMode configures whether servers request or require client certificates.
type ClientAuthMode string

const (
	// ClientAuthNone disables client certificate verification.
	ClientAuthNone ClientAuthMode = "none"
	// ClientAuthVerifyIfGiven verifies client certificates when clients present them.
	ClientAuthVerifyIfGiven ClientAuthMode = "verify_if_given"
	// ClientAuthRequireAndVerify requires and verifies client certificates.
	ClientAuthRequireAndVerify ClientAuthMode = "require_and_verify"
)

// ServerOptions configures server-side TLS and optional client-certificate verification.
type ServerOptions struct {
	// Cert contains the leaf server certificate.
	Cert *Material
	// Key contains the private key that matches Cert.
	Key *Material
	// CA optionally appends extra PEM certificates to the served chain.
	CA *Material
	// ClientCA contains the CA bundle used to verify client certificates for mTLS.
	ClientCA *Material
	// ServerName carries an explicit logical TLS name when needed by deployment tooling.
	ServerName string
	// ClientAuth selects the incoming client certificate policy.
	ClientAuth ClientAuthMode
}

// MaterialFromProto converts a TLS proto message into the shared runtime shape.
func MaterialFromProto(src *metacfg.TLSMaterial) *Material {
	if src == nil {
		return nil
	}
	out := &Material{}
	switch source := src.Source.(type) {
	case *metacfg.TLSMaterial_Path:
		out.Path = source.Path
	case *metacfg.TLSMaterial_InlinePem:
		out.InlinePEM = source.InlinePem
	}
	if out.Path == "" && out.InlinePEM == "" {
		return nil
	}
	return out
}

// MaterialToProto converts a shared TLS material shape into the proto model.
func MaterialToProto(src *Material) *metacfg.TLSMaterial {
	if src == nil {
		return nil
	}
	if src.Path != "" {
		return &metacfg.TLSMaterial{Source: &metacfg.TLSMaterial_Path{Path: src.Path}}
	}
	if src.InlinePEM != "" {
		return &metacfg.TLSMaterial{Source: &metacfg.TLSMaterial_InlinePem{InlinePem: src.InlinePEM}}
	}
	return nil
}

// ClientOptionsFromProto converts the proto client TLS model into shared runtime options.
func ClientOptionsFromProto(src *metacfg.ClientTLS) *ClientOptions {
	if src == nil {
		return nil
	}
	out := &ClientOptions{
		CA:                 MaterialFromProto(src.GetCa()),
		Cert:               MaterialFromProto(src.GetCert()),
		Key:                MaterialFromProto(src.GetKey()),
		ServerName:         src.GetServerName(),
		InsecureSkipVerify: src.GetInsecureSkipVerify(),
	}
	if out.CA == nil && out.Cert == nil && out.Key == nil && out.ServerName == "" && !out.InsecureSkipVerify {
		return nil
	}
	return out
}

// ClientOptionsToProto converts shared runtime client TLS options into the proto model.
func ClientOptionsToProto(src *ClientOptions) *metacfg.ClientTLS {
	if src == nil {
		return nil
	}
	return &metacfg.ClientTLS{
		Ca:                 MaterialToProto(src.CA),
		Cert:               MaterialToProto(src.Cert),
		Key:                MaterialToProto(src.Key),
		ServerName:         src.ServerName,
		InsecureSkipVerify: src.InsecureSkipVerify,
	}
}

// ServerOptionsFromProto converts the proto server TLS model into shared runtime options.
func ServerOptionsFromProto(src *metacfg.ServerTLS) *ServerOptions {
	if src == nil {
		return nil
	}
	out := &ServerOptions{
		Cert:       MaterialFromProto(src.GetCert()),
		Key:        MaterialFromProto(src.GetKey()),
		CA:         MaterialFromProto(src.GetCa()),
		ClientCA:   MaterialFromProto(src.GetClientCa()),
		ServerName: src.GetServerName(),
		ClientAuth: clientAuthModeFromProto(src.GetClientAuth()),
	}
	if out.Cert == nil && out.Key == nil && out.CA == nil && out.ClientCA == nil && out.ServerName == "" &&
		out.ClientAuth == ClientAuthNone {
		return nil
	}
	return out
}

// ServerOptionsToProto converts shared runtime server TLS options into the proto model.
func ServerOptionsToProto(src *ServerOptions) *metacfg.ServerTLS {
	if src == nil {
		return nil
	}
	return &metacfg.ServerTLS{
		Cert:       MaterialToProto(src.Cert),
		Key:        MaterialToProto(src.Key),
		Ca:         MaterialToProto(src.CA),
		ClientCa:   MaterialToProto(src.ClientCA),
		ServerName: src.ServerName,
		ClientAuth: clientAuthModeToProto(src.ClientAuth),
	}
}

// ClientConfig builds a tls.Config for outgoing connections.
func ClientConfig(opts *ClientOptions) (*tls.Config, error) {
	if opts == nil {
		return nil, nil
	}
	err := validateClientOptions(opts)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         opts.ServerName,
		InsecureSkipVerify: opts.InsecureSkipVerify, //nolint:gosec // Explicit SDK option for controlled development environments.
	}
	if opts.CA != nil {
		pool, err := loadCertPool(opts.CA)
		if err != nil {
			return nil, fmt.Errorf("load client tls ca: %w", err)
		}
		cfg.RootCAs = pool
	}
	if opts.Cert != nil {
		var cert tls.Certificate
		cert, err = loadKeyPair(opts.Cert, opts.Key, nil)
		if err != nil {
			return nil, fmt.Errorf("load client tls certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

// ServerConfig builds a tls.Config for inbound listeners.
func ServerConfig(opts *ServerOptions) (*tls.Config, error) {
	if opts == nil {
		return nil, nil
	}
	err := validateServerOptions(opts)
	if err != nil {
		return nil, err
	}
	var cert tls.Certificate
	cert, err = loadKeyPair(opts.Cert, opts.Key, opts.CA)
	if err != nil {
		return nil, fmt.Errorf("load server tls certificate: %w", err)
	}
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}
	if opts.ServerName != "" {
		cfg.ServerName = opts.ServerName
	}
	switch effectiveClientAuth(opts.ClientAuth) {
	case ClientAuthVerifyIfGiven:
		cfg.ClientAuth = tls.VerifyClientCertIfGiven
	case ClientAuthRequireAndVerify:
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		cfg.ClientAuth = tls.NoClientCert
	}
	if effectiveClientAuth(opts.ClientAuth) != ClientAuthNone {
		pool, err := loadCertPool(opts.ClientCA)
		if err != nil {
			return nil, fmt.Errorf("load server client ca: %w", err)
		}
		cfg.ClientCAs = pool
	}
	return cfg, nil
}

func validateClientOptions(opts *ClientOptions) error {
	if opts == nil {
		return nil
	}
	err := validateMaterial("ca", opts.CA)
	if err != nil {
		return err
	}
	err = validateMaterial("cert", opts.Cert)
	if err != nil {
		return err
	}
	err = validateMaterial("key", opts.Key)
	if err != nil {
		return err
	}
	if (opts.Cert == nil) != (opts.Key == nil) {
		return fmt.Errorf("client tls cert and key must be provided together")
	}
	return nil
}

func validateServerOptions(opts *ServerOptions) error {
	if opts == nil {
		return nil
	}
	mode := effectiveClientAuth(opts.ClientAuth)
	switch mode {
	case ClientAuthNone, ClientAuthVerifyIfGiven, ClientAuthRequireAndVerify:
	default:
		return fmt.Errorf("unsupported server tls client auth mode %q", mode)
	}
	err := validateMaterial("cert", opts.Cert)
	if err != nil {
		return err
	}
	err = validateMaterial("key", opts.Key)
	if err != nil {
		return err
	}
	err = validateMaterial("ca", opts.CA)
	if err != nil {
		return err
	}
	err = validateMaterial("client_ca", opts.ClientCA)
	if err != nil {
		return err
	}
	if opts.Cert == nil || opts.Key == nil {
		return fmt.Errorf("server tls cert and key are required")
	}
	if mode != ClientAuthNone && opts.ClientCA == nil {
		return fmt.Errorf("server tls client_ca is required when client auth is enabled")
	}
	return nil
}

func validateMaterial(name string, material *Material) error {
	if material == nil {
		return nil
	}
	if material.Path != "" && material.InlinePEM != "" {
		return fmt.Errorf("tls %s must set exactly one of path or inline_pem", name)
	}
	if material.Path == "" && material.InlinePEM == "" {
		return fmt.Errorf("tls %s must set path or inline_pem", name)
	}
	return nil
}

func loadCertPool(material *Material) (*x509.CertPool, error) {
	if material == nil {
		return nil, fmt.Errorf("tls material is nil")
	}
	pemBytes, err := readMaterial(material)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("append certs from pem")
	}
	return pool, nil
}

func loadKeyPair(certMaterial, keyMaterial, caMaterial *Material) (tls.Certificate, error) {
	certPEM, err := readMaterial(certMaterial)
	if err != nil {
		return tls.Certificate{}, err
	}
	if caMaterial != nil {
		var caPEM []byte
		caPEM, err = readMaterial(caMaterial)
		if err != nil {
			return tls.Certificate{}, err
		}
		certPEM = append(certPEM, '\n')
		certPEM = append(certPEM, caPEM...)
	}
	var keyPEM []byte

	keyPEM, err = readMaterial(keyMaterial)
	if err != nil {
		return tls.Certificate{}, err
	}
	var cert tls.Certificate
	cert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse x509 key pair: %w", err)
	}
	return cert, nil
}

func readMaterial(material *Material) ([]byte, error) {
	if material == nil {
		return nil, fmt.Errorf("tls material is nil")
	}
	if material.Path != "" {
		data, err := os.ReadFile(material.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", material.Path, err)
		}
		return data, nil
	}
	return []byte(material.InlinePEM), nil
}

func clientAuthModeFromProto(mode metacfg.ServerTLS_ClientAuth) ClientAuthMode {
	switch mode {
	case metacfg.ServerTLS_VERIFY_IF_GIVEN:
		return ClientAuthVerifyIfGiven
	case metacfg.ServerTLS_REQUIRE_AND_VERIFY:
		return ClientAuthRequireAndVerify
	default:
		return ClientAuthNone
	}
}

func clientAuthModeToProto(mode ClientAuthMode) metacfg.ServerTLS_ClientAuth {
	switch mode {
	case ClientAuthVerifyIfGiven:
		return metacfg.ServerTLS_VERIFY_IF_GIVEN
	case ClientAuthRequireAndVerify:
		return metacfg.ServerTLS_REQUIRE_AND_VERIFY
	default:
		return metacfg.ServerTLS_NONE
	}
}

func effectiveClientAuth(mode ClientAuthMode) ClientAuthMode {
	if mode == "" {
		return ClientAuthNone
	}
	return mode
}
