package nats

import "testing"

func TestPortFromEnvValidatesRange(t *testing.T) {
	const fallback = 4222
	for _, raw := range []string{"0", "65536", "not-a-port"} {
		got := PortFromEnv(raw, fallback)
		if got != fallback {
			t.Fatalf("PortFromEnv(%q) = %d, want %d", raw, got, fallback)
		}
	}
}
