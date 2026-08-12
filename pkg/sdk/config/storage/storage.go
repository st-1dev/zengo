package storage

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// ErrNotFound reports that no config file exists for the requested kind.
var ErrNotFound = errors.New("not found")

// Storage loads typed protobuf config objects by logical kind.
type Storage interface {
	// Get loads cfg from the config source registered for kind.
	Get(kind string, cfg proto.Message) error
}
