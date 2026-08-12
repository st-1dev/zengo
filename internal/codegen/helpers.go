package codegen

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"zengo/platform/sdk/tlsconfig"
)

// Funcs returns the shared template helper map for handwritten codegen templates.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"join":         strings.Join,
		"goName":       GoName,
		"clientTLS":    ClientTLSLiteral,
		"serverTLS":    ServerTLSLiteral,
		"ucFirst":      UcFirst,
		"isScalarType": IsScalarType,
		"quote":        Quote,
	}
}

// GoName converts snake_case field names into exported Go-style names.
func GoName(field string) string {
	parts := strings.Split(field, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

// UcFirst uppercases the first byte of s.
func UcFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// IsScalarType reports whether t is a scalar protobuf field type.
func IsScalarType(t string) bool {
	switch t {
	case "string",
		"bytes",
		"bool",
		"double",
		"float",
		"int32",
		"int64",
		"uint32",
		"uint64",
		"sint32",
		"sint64",
		"fixed32",
		"fixed64",
		"sfixed32",
		"sfixed64":
		return true
	default:
		return false
	}
}

// Quote returns a Go string literal for s.
func Quote(s string) string {
	return strconv.Quote(s)
}

// ClientTLSLiteral renders a Go literal for shared client TLS options.
func ClientTLSLiteral(opts *tlsconfig.ClientOptions) string {
	if opts == nil {
		return "nil"
	}
	return fmt.Sprintf("&tlsconfig.ClientOptions{CA: %s, Cert: %s, Key: %s, ServerName: %s, InsecureSkipVerify: %t}",
		materialLiteral(opts.CA),
		materialLiteral(opts.Cert),
		materialLiteral(opts.Key),
		Quote(opts.ServerName),
		opts.InsecureSkipVerify,
	)
}

// ServerTLSLiteral renders a Go literal for shared server TLS options.
func ServerTLSLiteral(opts *tlsconfig.ServerOptions) string {
	if opts == nil {
		return "nil"
	}
	return fmt.Sprintf(
		"&tlsconfig.ServerOptions{Cert: %s, Key: %s, CA: %s, ClientCA: %s, ServerName: %s, ClientAuth: tlsconfig.ClientAuthMode(%s)}",
		materialLiteral(opts.Cert),
		materialLiteral(opts.Key),
		materialLiteral(opts.CA),
		materialLiteral(opts.ClientCA),
		Quote(opts.ServerName),
		Quote(string(opts.ClientAuth)),
	)
}

func materialLiteral(material *tlsconfig.Material) string {
	if material == nil {
		return "nil"
	}
	return fmt.Sprintf("&tlsconfig.Material{Path: %s, InlinePEM: %s}", Quote(material.Path), Quote(material.InlinePEM))
}
