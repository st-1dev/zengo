package router_test

import (
	"testing"
	"zengo/platform/sdk/router"
)

func TestEventEnvelopeValidate(t *testing.T) {
	env := router.NewEventEnvelope("v1", "UserCreated", []byte(`{}`))
	err := env.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
