package kafka_test

import (
	"os"
	"path/filepath"
	"testing"
	"zengo/platform/sdk/config"

	platformkafka "zengo/platform/sdk/transport/queue/kafka"
)

func TestBrokersFromLoader(t *testing.T) {
	dir := t.TempDir()
	content := `kind: kafka
api_version: v1
spec:
  brokers:
    - " kafka:9092 "
    - kafka:9093
`
	err := os.WriteFile(filepath.Join(dir, "kafka.yaml"), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	got := platformkafka.BrokersFromLoader(config.NewLoader(dir), "kafka", nil)
	if len(got) != 2 || got[0] != "kafka:9092" || got[1] != "kafka:9093" {
		t.Fatalf("brokers = %v", got)
	}
}

func TestConfigFromLoaderBrokerFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		manifest []string
		env      string
		want     string
	}{
		{
			name:     "manifest",
			manifest: []string{" manifest:9092 "},
			env:      "env:9092",
			want:     "manifest:9092",
		},
		{
			name: "environment",
			env:  " env:9092 , env:9093 ",
			want: "env:9092",
		},
		{
			name: "default",
			want: "localhost:9092",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", tt.env)
			cfg := platformkafka.ConfigFromLoader(nil, "", tt.manifest)
			got := platformkafka.Brokers(cfg)
			if len(got) == 0 || got[0] != tt.want {
				t.Fatalf("brokers = %v, want first %q", got, tt.want)
			}
		})
	}
}
