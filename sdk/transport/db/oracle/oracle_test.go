package oracle

import (
	"context"
	"testing"

	oraclecfg "zengo/platform/api/config/db/oracle"
)

func TestDSNUsesDefaultPort(t *testing.T) {
	password := "tiger"
	cfg := &oraclecfg.Config{
		Spec: &oraclecfg.Spec{
			Host:        "db.example.com",
			ServiceName: "FREEPDB1",
			UserName:    "scott",
			Password:    &password,
		},
	}

	got := DSN(cfg)
	want := "oracle://scott:tiger@db.example.com:1521/FREEPDB1"
	if got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}

func TestDSNEscapesCredentialsAndService(t *testing.T) {
	password := "p@ss/word"
	port := int32(1522)
	cfg := &oraclecfg.Config{
		Spec: &oraclecfg.Spec{
			Host:        "db.example.com",
			Port:        &port,
			ServiceName: "ORCL/XE",
			UserName:    "user/name",
			Password:    &password,
		},
	}

	got := DSN(cfg)
	want := "oracle://user%2Fname:p@ss%2Fword@db.example.com:1522/ORCL%2FXE"
	if got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}

func TestOpenRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *oraclecfg.Config
		want string
	}{
		{name: "nil", cfg: nil, want: "oracle config is nil"},
		{
			name: "missing host",
			cfg:  &oraclecfg.Config{Spec: &oraclecfg.Spec{ServiceName: "FREEPDB1", UserName: "scott"}},
			want: "oracle host is required",
		},
		{
			name: "missing service",
			cfg:  &oraclecfg.Config{Spec: &oraclecfg.Spec{Host: "db.example.com", UserName: "scott"}},
			want: "oracle service_name is required",
		},
		{
			name: "missing user",
			cfg:  &oraclecfg.Config{Spec: &oraclecfg.Spec{Host: "db.example.com", ServiceName: "FREEPDB1"}},
			want: "oracle user_name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Connect(context.Background(), tt.cfg)
			if err == nil {
				if db != nil {
					_ = db.Close()
				}
				t.Fatalf("Open() error = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Fatalf("Open() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}
