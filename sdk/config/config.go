package config

import (
	"fmt"
	"zengo/platform/pkg/sdk/config/storage"
	"zengo/platform/pkg/sdk/config/storage/local"

	postgrescfg "zengo/platform/api/config/db/postgres"
	loggingcfg "zengo/platform/api/config/logging"

	"google.golang.org/protobuf/proto"
)

// Loader loads typed configuration objects by kind from a directory.
//
// Config files may be YAML (.yaml/.yml) or prototext (.textproto/.pbtxt); see
// zengo/platform/pkg/sdk/config/configfmt for the supported wire formats.
type Loader struct {
	storage storage.Storage
}

// NewLoader opens a loader rooted at path, typically configs/ in a service directory.
func NewLoader(path string) *Loader {
	return &Loader{storage: local.New(path)}
}

// Get loads cfg from the config file registered for kind.
//
// The caller must pass a writable protobuf message of the expected concrete
// type.
func (l *Loader) Get(kind string, cfg proto.Message) error {
	err := l.storage.Get(kind, cfg)
	if err != nil {
		return fmt.Errorf("load config kind %q: %w", kind, err)
	}
	return nil
}

// Postgres loads PostgreSQL connection config by kind.
func (l *Loader) Postgres(kind string) (*postgrescfg.Config, error) {
	cfg := &postgrescfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Logging loads logging config by kind.
func (l *Loader) Logging(kind string) (*loggingcfg.Config, error) {
	cfg := &loggingcfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
