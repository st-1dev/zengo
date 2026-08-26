package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"

	"zengo/platform/sdk/router"
)

type fakeConsumerGroupSession struct {
	sarama.ConsumerGroupSession
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func (s *fakeConsumerGroupSession) Context() context.Context {
	return s.ctx
}

func (s *fakeConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	s.marked = append(s.marked, msg)
}

type fakeConsumerGroupClaim struct {
	sarama.ConsumerGroupClaim
	messages <-chan *sarama.ConsumerMessage
}

func (c *fakeConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return c.messages
}

func TestConsumerGroupHandlerCommitSemantics(t *testing.T) {
	handlerErr := errors.New("handler failed")
	tests := []struct {
		name             string
		value            []byte
		handlerErr       error
		wantErr          error
		wantHandlerCalls int
		wantMarked       int
	}{
		{
			name:             "success",
			value:            []byte(`{"kind":"created"}`),
			wantHandlerCalls: 1,
			wantMarked:       1,
		},
		{
			name:             "handler error",
			value:            []byte(`{"kind":"created"}`),
			handlerErr:       handlerErr,
			wantErr:          handlerErr,
			wantHandlerCalls: 1,
		},
		{
			name:       "decode error",
			value:      []byte("{"),
			wantMarked: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := &sarama.ConsumerMessage{Topic: "events", Value: tc.value}
			messages := make(chan *sarama.ConsumerMessage, 1)
			messages <- msg
			close(messages)

			session := &fakeConsumerGroupSession{ctx: context.Background()}
			handlerCalls := 0
			handler := consumerGroupHandler{
				rootCtx: context.Background(),
				handler: func(context.Context, router.EventEnvelope) error {
					handlerCalls++
					return tc.handlerErr
				},
			}

			err := handler.ConsumeClaim(session, &fakeConsumerGroupClaim{messages: messages})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if handlerCalls != tc.wantHandlerCalls {
				t.Fatalf("handler calls = %d, want %d", handlerCalls, tc.wantHandlerCalls)
			}
			if len(session.marked) != tc.wantMarked {
				t.Fatalf("marked messages = %d, want %d", len(session.marked), tc.wantMarked)
			}
			if tc.wantMarked == 1 && session.marked[0] != msg {
				t.Fatal("marked a different message")
			}
		})
	}
}
