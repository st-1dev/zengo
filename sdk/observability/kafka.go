package observability

import (
	"context"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
)

type kafkaHeaderCarrier struct {
	headers []*sarama.RecordHeader
}

func (c kafkaHeaderCarrier) Get(key string) string {
	for _, h := range c.headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	for i, h := range c.headers {
		if string(h.Key) == key {
			c.headers[i].Value = []byte(value)
			return
		}
	}
	c.headers = append(c.headers, &sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func (c kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(c.headers))
	for i, h := range c.headers {
		keys[i] = string(h.Key)
	}
	return keys
}

// InjectKafkaHeaders injects W3C trace context into Kafka message headers.
func InjectKafkaHeaders(ctx context.Context, headers []*sarama.RecordHeader) []*sarama.RecordHeader {
	carrier := &kafkaHeaderCarrier{headers: headers}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.headers
}

// ExtractKafkaContext extracts W3C trace context from Kafka message headers.
func ExtractKafkaContext(ctx context.Context, headers []*sarama.RecordHeader) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, &kafkaHeaderCarrier{headers: headers})
}
