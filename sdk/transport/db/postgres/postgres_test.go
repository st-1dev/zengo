package postgres

import (
	"math"
	"testing"

	postgrescfg "zengo/platform/api/config/db/postgres"
)

func TestPoolConfigFromPortValidatesRange(t *testing.T) {
	for _, port := range []int{0, math.MaxUint16 + 1} {
		_, err := PoolConfigFromPort("localhost", port, "app", "user", "password")
		if err == nil {
			t.Fatalf("PoolConfigFromPort() port %d: expected error", port)
		}
	}
}

func TestNewPoolRejectsConnectionOverflow(t *testing.T) {
	cfg := &postgrescfg.Config{
		Spec: &postgrescfg.Spec{
			Connection: &postgrescfg.Connection{
				MaxOpen: math.MaxInt32 + 1,
			},
		},
	}
	_, err := NewPool(t.Context(), cfg)
	if err == nil {
		t.Fatal("expected max_open overflow error")
	}
}

func TestPortFromEnvValidatesRange(t *testing.T) {
	const fallback = 5432
	for _, raw := range []string{"0", "65536", "not-a-port"} {
		got := PortFromEnv(raw, fallback)
		if got != fallback {
			t.Fatalf("PortFromEnv(%q) = %d, want %d", raw, got, fallback)
		}
	}
}
