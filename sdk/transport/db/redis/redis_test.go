package redis

import (
	"math"
	"testing"
)

func TestClientFromPortValidatesNumericRanges(t *testing.T) {
	tests := []struct {
		name string
		port int
		db   int
	}{
		{name: "invalid port", port: math.MaxUint16 + 1},
		{name: "negative db", port: 6379, db: -1},
		{name: "db overflow", port: 6379, db: math.MaxInt32 + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClientFromPort(t.Context(), "localhost", tt.port, tt.db)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
