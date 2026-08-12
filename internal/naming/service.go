package naming

import (
	"strings"
	"unicode"
)

// HandlerPackage normalizes a service name into a valid Go/proto package segment.
func HandlerPackage(serviceName string) string {
	base := strings.TrimSpace(strings.ToLower(serviceName))
	base = strings.TrimSuffix(base, "-service")
	base = strings.TrimSpace(base)
	if base == "" {
		return "app"
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range base {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if b.Len() == 0 && unicode.IsDigit(r) {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	normalized := strings.Trim(b.String(), "_")
	if normalized == "" {
		return "app"
	}
	return normalized
}

// HandlerTitle converts a normalized handler package into a Go/proto type prefix.
func HandlerTitle(handlerPkg string) string {
	parts := strings.FieldsFunc(handlerPkg, func(r rune) bool {
		return r == '_' || r == '-' || !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return "App"
	}

	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		b.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "App"
	}
	return b.String()
}
