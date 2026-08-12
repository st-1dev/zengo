package versioning

// Field describes one protobuf message field as seen by the versioning loader.
type Field struct {
	// Name is the canonical field name in the loaded schema.
	Name string
	// LegacyName is the optional legacy name mapped from zengo field metadata.
	LegacyName string // hub-only: zengo.options.field_map.legacy_name
	// Number is the protobuf field number.
	Number int
	// Type is the flattened field type name used by the generator.
	Type string
	// Repeated reports whether the field is repeated.
	Repeated bool
}

// Message describes one protobuf message in the loaded schema.
type Message struct {
	// Name is the protobuf message name.
	Name string
	// Fields are the protobuf fields in declaration order.
	Fields []Field
	// File is the import path that contributed the message.
	File string
}

// RPC describes one protobuf RPC in the loaded schema.
type RPC struct {
	// Name is the RPC method name.
	Name string
	// RequestType is the request message type name.
	RequestType string
	// ResponseType is the response message type name.
	ResponseType string
	// Service is the owning service name.
	Service string
	// File is the import path that contributed the RPC.
	File string
}

// Service describes one protobuf service in the loaded schema.
type Service struct {
	// Name is the protobuf service name.
	Name string
	// RPCs are the service methods discovered by the loader.
	RPCs []RPC
	// File is the import path that contributed the service.
	File string
}

// Schema is the normalized API view used by compatibility planning.
type Schema struct {
	// Package is the import path or package identifier used for the schema.
	Package string
	// Messages maps message names to their normalized representation.
	Messages map[string]Message
	// Services contains normalized service declarations.
	Services []Service
	// Files records the contributing import paths or files for the schema.
	Files []string
}
