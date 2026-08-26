# Kafka Handler Failure Commit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Не подтверждать Kafka-сообщение, если бизнес-handler завершился ошибкой, и вернуть эту ошибку Sarama для последующей повторной доставки.

**Architecture:** Изменение остаётся внутри `consumerGroupHandler`: `consumeMessage` возвращает handler error, а `ConsumeClaim` немедленно передаёт его вызывающей стороне. Успешный и decode-error пути сохраняют текущую маркировку offset; новые публичные API, DLQ и зависимости не добавляются.

**Tech Stack:** Go 1.26, `github.com/IBM/sarama` v1.50.1, существующие `policy.Executor`, `router.EventEnvelope` и стандартный `testing`.

**Spec:** `docs/superpowers/specs/2026-08-26-kafka-commit-design.md`

**Issue:** https://github.com/st-1dev/zengo/issues/1

## Global Constraints

- Не менять публичные типы и функции package `kafka`.
- Handler error никогда не вызывает `MarkMessage`; success и malformed envelope вызывают его ровно один раз.
- Malformed envelope остаётся текущим poison-pill случаем: журналирование, mark и продолжение claim.
- Сохранить fallback на `policy.NewExecutor(policy.Options{})`, когда `consumerGroupHandler.exec == nil`.
- Не добавлять DLQ, новые callbacks, бесконечные retries или зависимости.

---

### Task 1: Передать handler error из consumer group без commit

**Files:**
- Create: `sdk/transport/queue/kafka/kafka_internal_test.go`
- Modify: `sdk/transport/queue/kafka/kafka.go:331-398`

**Interfaces:**
- Consumes: `HandlerFunc`, `sarama.ConsumerGroupSession`, `sarama.ConsumerGroupClaim`, `policy.Executor.Do`.
- Produces: `consumerGroupHandler.consumeMessage(sess sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error`; `ConsumeClaim` возвращает эту ошибку без маркировки сообщения.

- [ ] **Step 1: Написать regression test для трёх commit-семантик**

Создать `sdk/transport/queue/kafka/kafka_internal_test.go`:

```go
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
```

- [ ] **Step 2: Запустить тест и подтвердить RED**

Run:

```bash
go test ./sdk/transport/queue/kafka -run '^TestConsumerGroupHandlerCommitSemantics$' -count=1
```

Expected: FAIL в subtest `handler_error`: текущий `ConsumeClaim` возвращает `nil` и помечает сообщение вместо возврата `handler failed`.

- [ ] **Step 3: Реализовать минимальную передачу ошибки**

В `sdk/transport/queue/kafka/kafka.go` заменить два метода следующим кодом:

```go
func (h consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-h.rootCtx.Done():
			return nil
		case <-sess.Context().Done():
			return nil
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if err := h.consumeMessage(sess, msg); err != nil {
				return err
			}
		}
	}
}

func (h consumerGroupHandler) consumeMessage(
	sess sarama.ConsumerGroupSession,
	msg *sarama.ConsumerMessage,
) error {
	ctx := observability.ExtractKafkaContext(h.rootCtx, msg.Headers)
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("messaging.system", "kafka"),
		observability.StringAttribute("messaging.operation", "receive"),
		observability.StringAttribute("messaging.destination.name", msg.Topic),
	)
	ctx = spanCtx
	defer endSpan()

	var env router.EventEnvelope
	err := json.Unmarshal(msg.Value, &env)
	if err != nil {
		observability.RecordException(ctx, err, "decode kafka envelope")
		slog.Error(
			"kafka decode envelope failed",
			"topic",
			msg.Topic,
			"partition",
			msg.Partition,
			"offset",
			msg.Offset,
			"err",
			err,
		)
		sess.MarkMessage(msg, "")
		return nil
	}
	exec := h.exec
	if exec == nil {
		exec = policy.NewExecutor(policy.Options{})
	}
	err = exec.Do(ctx, func(callCtx context.Context) error {
		return h.handler(callCtx, env)
	})
	if err != nil {
		observability.RecordException(ctx, err, "handle kafka message")
		slog.Error(
			"kafka handler failed",
			"topic",
			msg.Topic,
			"partition",
			msg.Partition,
			"offset",
			msg.Offset,
			"err",
			err,
		)
		return err
	}
	sess.MarkMessage(msg, "")
	return nil
}
```

- [ ] **Step 4: Отформатировать изменённые Go-файлы**

Run:

```bash
gofmt -w sdk/transport/queue/kafka/kafka.go sdk/transport/queue/kafka/kafka_internal_test.go
```

- [ ] **Step 5: Запустить regression test и подтвердить GREEN**

Run:

```bash
go test ./sdk/transport/queue/kafka -run '^TestConsumerGroupHandlerCommitSemantics$' -count=1
```

Expected: PASS; success и decode error имеют один mark, handler error возвращается без mark.

- [ ] **Step 6: Проверить package и весь repository**

Run:

```bash
go test -race ./sdk/transport/queue/kafka -count=1
go test ./...
```

Expected: обе команды PASS.

- [ ] **Step 7: Зафиксировать implementation commit**

```bash
git add sdk/transport/queue/kafka/kafka.go sdk/transport/queue/kafka/kafka_internal_test.go
git commit -m "fix(kafka): preserve messages when handlers fail"
```
