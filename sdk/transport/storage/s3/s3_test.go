package s3

import (
	"math"
	"testing"
)

func TestClientFromMinIOValidatesPort(t *testing.T) {
	_, err := ClientFromMinIO(t.Context(), "localhost", math.MaxUint16+1, "bucket")
	if err == nil {
		t.Fatal("expected port range error")
	}
}

func TestPortFromEnvValidatesRange(t *testing.T) {
	const fallback = 9000
	for _, raw := range []string{"0", "65536", "not-a-port"} {
		got := PortFromEnv(raw, fallback)
		if got != fallback {
			t.Fatalf("PortFromEnv(%q) = %d, want %d", raw, got, fallback)
		}
	}
}
