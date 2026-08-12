package router

import (
	"fmt"
)

// EventEnvelope is the Kafka event wrapper with apiVersion/kind metadata.
type EventEnvelope struct {
	// APIVersion is the logical API version carried by the event payload.
	APIVersion string `json:"apiVersion"`
	// Kind is the logical event kind or method name carried by the envelope.
	Kind string `json:"kind"`
	// Payload is the raw event body encoded by the producer.
	Payload []byte `json:"payload"`
}

// NewEventEnvelope creates a validated envelope payload wrapper for transport use.
func NewEventEnvelope(apiVersion, kind string, payload []byte) EventEnvelope {
	return EventEnvelope{
		APIVersion: apiVersion,
		Kind:       kind,
		Payload:    payload,
	}
}

// Validate reports whether the envelope contains the required routing metadata.
func (e EventEnvelope) Validate() error {
	if e.APIVersion == "" {
		return fmt.Errorf("event envelope: apiVersion is required")
	}
	if e.Kind == "" {
		return fmt.Errorf("event envelope: kind is required")
	}
	return nil
}
